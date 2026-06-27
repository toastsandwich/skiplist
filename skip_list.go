// Package skiplist is a sorted []byte key/value store.
//
// Put, Get, and Pop are O(log n) on average. Keys sort with bytes.Compare.
//
// Use SkipList for single-threaded code. Use SyncSkipList when goroutines
// share the same map.
//
//	s := NewSkipList(0, 0) // defaults: max level 32, p 0.5
//	s.Put([]byte("a"), []byte("1"))
//	v, _ := s.Get([]byte("a"))
//	s.Pop([]byte("a"))
//	for k, v := range s.All() { _ = k; _ = v }
//
//	safe := NewSyncSkipList(0, 0)
//	safe.Put([]byte("a"), []byte("1"))
//	v, _ = safe.Get([]byte("a"))
package skiplist

import (
	"bytes"
	"errors"
	"iter"
	"math"
	"math/bits"
	"math/rand/v2"
	"sync"
)

var (
	ErrNilKey       = errors.New("key cannot be nil or zero len slice of byte")
	ErrNilVal       = errors.New("value cannot be nil or zero len slice of byte")
	ErrKeyNotFound  = errors.New("key not found")
	ErrSkiplistFull = errors.New("skip list is full")
)

// DefaultValues returns the default max level (32) and probability (0.5).
func DefaultValues() (int, float64) {
	return 32, 0.5
}

// MaxEntries returns how many entries maxLevel and p can hold: (1/p)^maxLevel.
// For p=0.5 and maxLevel=32 this is 2^32.
func MaxEntries(maxLevel int, p float64) int64 {
	if maxLevel <= 0 || p <= 0 || p >= 1 {
		return 0
	}
	if p == 0.5 {
		if maxLevel >= 63 {
			return math.MaxInt64
		}
		return int64(1) << maxLevel
	}
	v := math.Pow(1/p, float64(maxLevel))
	if v > math.MaxInt64 {
		return math.MaxInt64
	}
	return int64(math.Round(v))
}

// MaxLevelFor returns the minimum max level needed for n entries at probability p:
// ceil(log_{1/p}(n)).
func MaxLevelFor(n int64, p float64) int {
	if n <= 1 {
		return 1
	}
	if p <= 0 || p >= 1 {
		return 32
	}
	return int(math.Ceil(math.Log(float64(n)) / math.Log(1/p)))
}

// Element is one node in the list. Most callers use Put instead.
type Element struct {
	Key, Value []byte
	Level      int

	// Unexported forward pointers for each level.
	nexts    []*Element
	nextsLen int
}

func (e *Element) prepare(k, v []byte, lvl int) {
	e.Key = k
	e.Value = v
	e.Level = lvl

	need := lvl + 1
	if need > cap(e.nexts) {
		e.nexts = make([]*Element, need)
	} else {
		e.nexts = e.nexts[:need]
		for i := range need {
			e.nexts[i] = nil
		}
	}
}

func (e *Element) Reset() *Element {
	e.Key = nil
	e.Value = nil
	e.Level = 0
	return e
}

// make a header element for skiplist
func makeHeaderElement(max int) *Element {
	e := &Element{
		nexts:    make([]*Element, max),
		nextsLen: max,
	}

	// initially all headers should point to nil
	for i := range e.nexts {
		e.nexts[i] = nilElement
	}
	return e
}

// nilElement is the last element which will just reference
// end of the skiplist, it does not have any next elements
var nilElement = &Element{}

// SetLevel sets the node height. Does not update links.
func (e *Element) SetLevel(l int) {
	e.Level = l
}

// NewElement builds a node with level l. Use Put for normal inserts.
func (s *SkipList) NewElement(key, val []byte, l int) *Element {
	return &Element{
		Key:   key,
		Value: val,
		Level: l,
		nexts: make([]*Element, l+1),
	}
}

// SkipList stores sorted []byte key/value pairs.
type SkipList struct {
	Header   *Element
	MaxLevel int
	P        float64

	// MaxLen is the max number of entries. 0 means unlimited.
	// Theoretical max at current MaxLevel and P is MaxEntries(MaxLevel, P).
	MaxLen int64

	// level is the highest non-empty tower (-1 when empty).
	level int

	nilElement *Element
	len        int
	// used to get update lists
	pool sync.Pool
}

// NewSkipList creates a list. Pass 0 for either arg to use defaults (32, 0.5).
func NewSkipList(maxlevel int, p float64) *SkipList {
	if maxlevel <= 0 {
		maxlevel = 32
	}
	if p <= 0 {
		p = 0.5
	}
	return &SkipList{
		Header:     makeHeaderElement(maxlevel),
		MaxLevel:   maxlevel,
		MaxLen:     MaxEntries(maxlevel, p),
		level:      -1,
		nilElement: nilElement,
		P:          p,
		pool: sync.Pool{
			New: func() any {
				return make([]*Element, maxlevel)
			},
		},
	}
}

// topLevel returns the highest level to search, optionally raised by hint.
func (s *SkipList) topLevel(hint int) int {
	top := s.level
	if hint > top {
		top = hint
	}
	if top < 0 {
		return 0
	}
	return top
}

func (s *SkipList) fillUpdate(update []*Element, key []byte, top int) {
	x := s.Header
	for i := top; i >= 0; i-- {
		for bytes.Compare(x.nexts[i].Key, key) < 0 && x.nexts[i] != s.nilElement {
			x = x.nexts[i]
		}
		update[i] = x
	}
}

// Get returns the value for key. ErrKeyNotFound if missing, ErrNilKey if empty.
func (s *SkipList) Get(key []byte) ([]byte, error) {
	if len(key) == 0 {
		return nil, ErrNilKey
	}
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

func validateKeyValue(key []byte, value []byte) error {
	if len(key) == 0 {
		return ErrNilKey
	}
	if len(value) == 0 {
		return ErrNilVal
	}
	return nil
}

// Put inserts or updates a key/value pair.
// ErrNilKey or ErrNilVal on empty input. ErrSkiplistFull if MaxLen is reached.
func (s *SkipList) Put(key, val []byte) error {
	if err := validateKeyValue(key, val); err != nil {
		return err
	}

	update := s.pool.Get().([]*Element)
	defer s.pool.Put(update)

	searchTop := s.topLevel(0)
	s.fillUpdate(update, key, searchTop)

	x := update[0].nexts[0]
	if bytes.Equal(x.Key, key) {
		x.Value = val
		return nil
	}
	if s.MaxLen > 0 && int64(s.len) >= s.MaxLen {
		return ErrSkiplistFull
	}
	lvl := s.randomLevel()
	if lvl > searchTop {
		s.fillUpdate(update, key, lvl)
	}
	x = s.NewElement(key, val, lvl)
	for i := 0; i <= lvl; i++ {
		x.nexts[i] = update[i].nexts[i]
		update[i].nexts[i] = x
	}
	if lvl > s.level {
		s.level = lvl
	}
	s.len++
	return nil
}

// Pop removes key. ErrKeyNotFound if missing, ErrNilKey if empty.
func (s *SkipList) Pop(key []byte) error {
	if len(key) == 0 {
		return ErrNilKey
	}
	update := s.pool.Get().([]*Element)
	defer s.pool.Put(update)

	s.fillUpdate(update, key, s.topLevel(0))

	x := update[0].nexts[0]
	if x == s.nilElement || !bytes.Equal(x.Key, key) {
		return ErrKeyNotFound
	}

	removedLevel := x.Level

	// Unlink the node from all the levels it participates in.
	// A node with Level=l only has forward pointers (and is linked) at levels 0..l.
	for i := 0; i <= x.Level && i < s.MaxLevel; i++ {
		if update[i].nexts[i] == x {
			update[i].nexts[i] = x.nexts[i]
		}
	}

	if removedLevel == s.level {
		for s.level >= 0 && s.Header.nexts[s.level] == s.nilElement {
			s.level--
		}
	}

	s.len--
	return nil
}

// All iterates key/value pairs in sorted order. Don't modify the list while ranging.
func (s *SkipList) All() iter.Seq2[[]byte, []byte] {
	return func(yield func([]byte, []byte) bool) {
		for i := s.Header.nexts[0]; i != s.nilElement; i = i.nexts[0] {
			if !yield(i.Key, i.Value) {
				return
			}
		}
	}
}

// ForEach walks all pairs in sorted order. Return false from do to stop early.
func (s *SkipList) ForEach(do func(key, value []byte) bool) {
	for i := s.Header.nexts[0]; i != s.nilElement; i = i.nexts[0] {
		if !do(i.Key, i.Value) {
			return
		}
	}
}

// Len returns the number of entries.
func (s *SkipList) Len() int {
	return s.len
}

// Capacity returns the enforced entry limit when MaxLen is set.
// When MaxLen is 0 (unlimited), returns MaxEntries(MaxLevel, P).
func (s *SkipList) Cap() int64 {
	if s.MaxLen > 0 {
		return s.MaxLen
	}
	return MaxEntries(s.MaxLevel, s.P)
}

func (s *SkipList) randomLevel() int {
	// special case P = 0.5
	// trailing zeros can be considered as level assigned
	// xxxx1 = 0.5   level 0
	// xxx10 = 0.25  level 1
	// xx100 = 0.125 level 2
	if s.P == 0.5 {
		z := bits.TrailingZeros64(rand.Uint64())
		if z >= s.MaxLevel {
			return s.MaxLevel - 1
		}
		return z
	}
	l := 0
	for l < s.MaxLevel-1 && rand.Float64() < s.P {
		l++
	}
	return l
}
