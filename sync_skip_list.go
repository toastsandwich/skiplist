// SyncSkipList is the mutex-protected skip list.
//
// Same operations as SkipList, safe to call from many goroutines.
// Put takes ownership of the key and value slices — do not use them
// after Put returns.
//
// There is no All() or ForEach() yet; use Get in a loop or add your
// own iteration under RLock if you need a full scan.
package skiplist

import (
	"bytes"
	"math/bits"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type updateList struct {
	s []*Element
}

// SyncSkipList is a sorted []byte map safe for concurrent use.
type SyncSkipList struct {
	mu     sync.RWMutex
	Header *Element

	MaxLevel int
	P        float64

	MaxLen int64

	// level is the highest non-empty tower (-1 when empty).
	level int

	nilElement *Element
	len        atomic.Int64

	rmu sync.Mutex
	r   *rand.Rand

	elementPool pool[*Element]
	updatePool  pool[*updateList]
}

// NewSyncSkipList creates a concurrent skip list.
// Pass 0 for either argument to use defaults (32, 0.5).
// MaxLen is set to the theoretical max for those settings.
func NewSyncSkipList(maxlevel int, p float64) *SyncSkipList {
	if maxlevel <= 0 {
		maxlevel = 32
	}
	if p <= 0 {
		p = 0.5
	}

	maxLen := MaxEntries(maxlevel, p)

	return &SyncSkipList{
		mu:         sync.RWMutex{},
		Header:     makeHeaderElement(maxlevel),
		nilElement: nilElement,
		MaxLevel:   maxlevel,
		MaxLen:     maxLen,
		level:      -1,
		P:          p,

		r: rand.New(rand.NewSource(time.Now().Unix())),

		elementPool: pool[*Element]{new: func() *Element { return &Element{} }},
		updatePool: pool[*updateList]{new: func() *updateList {
			return &updateList{
				s: make([]*Element, maxlevel),
			}
		}},
	}
}

func (s *SyncSkipList) topLevel(hint int) int {
	top := max(s.level, hint)
	if top < 0 {
		return 0
	}
	return top
}

func (s *SyncSkipList) fillUpdate(update *updateList, key []byte, top int) {
	x := s.Header
	for i := top; i >= 0; i-- {
		for x.nexts[i] != s.nilElement && bytes.Compare(x.nexts[i].Key, key) < 0 {
			x = x.nexts[i]
		}
		update.s[i] = x
	}
}

// Get returns the value for key.
// Safe for concurrent readers. Zero allocs on hit.
// Returns ErrKeyNotFound or ErrNilKey.
func (s *SyncSkipList) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrNilKey
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	h := s.Header
	for l := s.topLevel(0); l >= 0; l-- {
		for bytes.Compare(h.nexts[l].Key, key) < 0 && h.nexts[l] != s.nilElement {
			h = h.nexts[l]
		}
	}
	h = h.nexts[0]
	if bytes.Equal(h.Key, key) {
		return h.Value, nil
	}
	return nil, ErrKeyNotFound
}

// Put inserts or updates a key/value pair.
// Takes ownership of key and val — do not read or write those slices afterward.
// Returns ErrNilKey, ErrNilVal, or ErrSkiplistFull.
func (s *SyncSkipList) Put(key, val []byte) error {
	if err := validateKeyValue(key, val); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	update := s.updatePool.Get()
	defer s.updatePool.Put(update)

	searchTop := s.topLevel(0)
	s.fillUpdate(update, key, searchTop)

	x := update.s[0].nexts[0]
	if bytes.Equal(x.Key, key) {
		x.Value = val
		return nil
	}

	if s.MaxLen > 0 && s.len.Load() >= s.MaxLen {
		return ErrSkiplistFull
	}
	lvl := s.randomLevel()
	if lvl > searchTop {
		s.fillUpdate(update, key, lvl)
	}

	x = s.elementPool.Get()
	x.prepare(key, val, lvl)

	for i := 0; i <= lvl; i++ {
		x.nexts[i] = update.s[i].nexts[i]
		update.s[i].nexts[i] = x
	}
	if lvl > s.level {
		s.level = lvl
	}
	s.len.Add(1)
	return nil
}

// Pop removes key.
// Returns ErrKeyNotFound or ErrNilKey.
func (s *SyncSkipList) Pop(key []byte) error {
	if len(key) == 0 {
		return ErrNilKey
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	update := s.updatePool.Get()
	defer s.updatePool.Put(update)

	s.fillUpdate(update, key, s.topLevel(0))

	x := update.s[0].nexts[0]
	if x == s.nilElement || !bytes.Equal(x.Key, key) {
		return ErrKeyNotFound
	}

	removedLevel := x.Level

	// Unlink the node from all the levels it participates in.
	// A node with Level=l only has forward pointers (and is linked) at levels 0..l.
	for i := 0; i <= x.Level && i < s.MaxLevel; i++ {
		if update.s[i].nexts[i] == x {
			update.s[i].nexts[i] = x.nexts[i]
		}
	}
	if removedLevel == s.level {
		for s.level >= 0 && s.Header.nexts[s.level] == s.nilElement {
			s.level--
		}
	}
	s.elementPool.Put(x.Reset()) // send delete element to pool

	s.len.Add(-1)
	return nil
}

// Len returns the number of entries.
func (s *SyncSkipList) Len() int64 {
	return s.len.Load()
}

// Cap returns the entry limit (MaxLen).
func (s *SyncSkipList) Cap() int64 {
	if s.MaxLen > 0 {
		return s.MaxLen
	}
	return MaxEntries(s.MaxLevel, s.P)
}

func (s *SyncSkipList) randomLevel() int {
	// special case P = 0.5
	// trailing zeros can be considered as level assigned
	// xxxx1 = 0.5   level 0
	// xxx10 = 0.25  level 1
	// xx100 = 0.125 level 2

	s.rmu.Lock()
	defer s.rmu.Unlock()

	if s.P == 0.5 {
		z := bits.TrailingZeros64(s.r.Uint64())
		if z >= s.MaxLevel {
			return s.MaxLevel - 1
		}
		return z
	}
	l := 0
	for l < s.MaxLevel-1 && s.r.Float64() < s.P {
		l++
	}
	return l
}
