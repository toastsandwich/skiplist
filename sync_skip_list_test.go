package skiplist

import (
	"bytes"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestSyncSkipList_PutGet(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if err := s.Put([]byte("hello"), []byte("world")); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		val, err := s.Get([]byte("hello"))
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !bytes.Equal(val, []byte("world")) {
			t.Errorf("expected world, got %q", val)
		}
	})

	t.Run("Update", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if err := s.Put([]byte("k1"), []byte("v1")); err != nil {
			t.Fatalf("Put failed: %v", err)
		}
		if err := s.Put([]byte("k1"), []byte("v2")); err != nil {
			t.Fatalf("Put update failed: %v", err)
		}

		val, err := s.Get([]byte("k1"))
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if !bytes.Equal(val, []byte("v2")) {
			t.Error("update did not replace value")
		}
		if s.Len() != 1 {
			t.Errorf("Len() = %d after update, want 1", s.Len())
		}
	})

	t.Run("Missing", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if _, err := s.Get([]byte("nope")); err != ErrKeyNotFound {
			t.Errorf("expected ErrKeyNotFound, got %v", err)
		}
		if _, err := s.Get(nil); err != ErrNilKey {
			t.Errorf("Get(nil): got %v, want ErrNilKey", err)
		}
		if _, err := s.Get([]byte{}); err != ErrNilKey {
			t.Errorf("Get(empty): got %v, want ErrNilKey", err)
		}
	})

	t.Run("Validation", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if err := s.Put(nil, []byte("v")); err != ErrNilKey {
			t.Errorf("Put(nil key): got %v, want ErrNilKey", err)
		}
		if err := s.Put([]byte("k"), nil); err != ErrNilVal {
			t.Errorf("Put(nil val): got %v, want ErrNilVal", err)
		}
	})
}

func TestSyncSkipList_Pop(t *testing.T) {
	t.Run("Existing", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		for _, kv := range [][2]string{{"a", "1"}, {"b", "2"}, {"c", "3"}} {
			if err := s.Put([]byte(kv[0]), []byte(kv[1])); err != nil {
				t.Fatalf("Put %s: %v", kv[0], err)
			}
		}

		if err := s.Pop([]byte("b")); err != nil {
			t.Errorf("Pop b: got %v", err)
		}
		if _, err := s.Get([]byte("b")); err != ErrKeyNotFound {
			t.Error("key b should be gone after pop")
		}
		va, err := s.Get([]byte("a"))
		if err != nil || !bytes.Equal(va, []byte("1")) {
			t.Error("a missing")
		}
		vc, err := s.Get([]byte("c"))
		if err != nil || !bytes.Equal(vc, []byte("3")) {
			t.Error("c missing")
		}
		if s.Len() != 2 {
			t.Errorf("Len() = %d, want 2", s.Len())
		}
	})

	t.Run("Missing", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if err := s.Put([]byte("x"), []byte("y")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := s.Pop([]byte("z")); err != ErrKeyNotFound {
			t.Errorf("pop missing: got %v, want ErrKeyNotFound", err)
		}
		if err := s.Pop(nil); err != ErrNilKey {
			t.Errorf("pop nil: got %v, want ErrNilKey", err)
		}
		if s.Len() != 1 {
			t.Errorf("Len() = %d after pop missing, want 1", s.Len())
		}
	})
}

func TestSyncSkipList_Len(t *testing.T) {
	t.Run("NewListIsZero", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if s.Len() != 0 {
			t.Errorf("Len() = %d, want 0", s.Len())
		}
	})

	t.Run("IncreasesOnInsert", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		_ = s.Put([]byte("a"), []byte("1"))
		_ = s.Put([]byte("b"), []byte("2"))
		if s.Len() != 2 {
			t.Errorf("Len() = %d, want 2", s.Len())
		}
	})

	t.Run("DecreasesOnPop", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		_ = s.Put([]byte("a"), []byte("1"))
		_ = s.Put([]byte("b"), []byte("2"))
		if err := s.Pop([]byte("b")); err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if s.Len() != 1 {
			t.Errorf("Len() = %d, want 1", s.Len())
		}
	})

	t.Run("PopOnEmptyDoesNotGoNegative", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if err := s.Pop([]byte("anything")); err != ErrKeyNotFound {
			t.Errorf("Pop on empty: got %v, want ErrKeyNotFound", err)
		}
		if s.Len() != 0 {
			t.Errorf("Len() = %d after pop on empty, want 0", s.Len())
		}
	})
}

func TestSyncSkipList_Capacity(t *testing.T) {
	s := NewSyncSkipList(16, 0.5)
	if s.Cap() != MaxEntries(16, 0.5) {
		t.Errorf("Cap() = %d, want %d", s.Cap(), MaxEntries(16, 0.5))
	}
}

func TestSyncSkipList_MaxLen(t *testing.T) {
	// maxLevel=1, p=0.5 => capacity of 2 entries
	s := NewSyncSkipList(1, 0.5)

	if err := s.Put([]byte("a"), []byte("1")); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := s.Put([]byte("b"), []byte("2")); err != nil {
		t.Fatalf("Put b: %v", err)
	}
	if err := s.Put([]byte("c"), []byte("3")); err != ErrSkiplistFull {
		t.Fatalf("Put c: got %v, want ErrSkiplistFull", err)
	}
	if s.Len() != 2 {
		t.Errorf("Len() = %d, want 2", s.Len())
	}

	if err := s.Put([]byte("a"), []byte("1-updated")); err != nil {
		t.Fatalf("Put update when full: %v", err)
	}
	if v, err := s.Get([]byte("a")); err != nil || string(v) != "1-updated" {
		t.Errorf("Get after update when full: val=%q err=%v", v, err)
	}
}

func TestSyncSkipList_CorrectnessLarge(t *testing.T) {
	const N = 5000
	s := NewSyncSkipList(32, 0.5)

	keys := make([][]byte, N)
	vals := make([][]byte, N)
	for i := range N {
		keys[i] = []byte(fmt.Sprintf("key-%08d", i))
		vals[i] = []byte(fmt.Sprintf("val-%08d", i))
		if err := s.Put(keys[i], vals[i]); err != nil {
			t.Fatalf("Put failed at %d: %v", i, err)
		}
	}

	for i := range N {
		got, err := s.Get(keys[i])
		if err != nil || !bytes.Equal(got, vals[i]) {
			t.Fatalf("get mismatch at %d", i)
		}
	}
	if s.Len() != N {
		t.Fatalf("Len() = %d, want %d", s.Len(), N)
	}
}

func TestSyncSkipList_ConcurrentPutGet(t *testing.T) {
	const (
		workers = 32
		per     = 200
	)
	s := NewSyncSkipList(32, 0.5)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(id int) {
			defer wg.Done()
			for i := range per {
				key := []byte(fmt.Sprintf("w%02d-key-%06d", id, i))
				val := []byte(fmt.Sprintf("w%02d-val-%06d", id, i))
				if err := s.Put(key, val); err != nil {
					t.Errorf("Put worker %d item %d: %v", id, i, err)
					return
				}
				got, err := s.Get(key)
				if err != nil || !bytes.Equal(got, val) {
					t.Errorf("Get worker %d item %d: got %q err=%v", id, i, got, err)
					return
				}
			}
		}(w)
	}
	wg.Wait()

	want := int64(workers * per)
	if s.Len() != want {
		t.Errorf("Len() = %d, want %d", s.Len(), want)
	}
}

func TestSyncSkipList_ConcurrentReadersWriters(t *testing.T) {
	const (
		writers = 8
		readers = 16
		ops     = 500
	)
	s := NewSyncSkipList(32, 0.5)

	// Preload some keys
	for i := range 100 {
		k := []byte(fmt.Sprintf("preload-%04d", i))
		if err := s.Put(k, k); err != nil {
			t.Fatalf("preload: %v", err)
		}
	}

	var wg sync.WaitGroup
	var readErrors atomic.Int64

	wg.Add(writers + readers)
	for w := range writers {
		go func(id int) {
			defer wg.Done()
			for i := range ops {
				key := []byte(fmt.Sprintf("w%02d-%06d", id, i))
				val := []byte(fmt.Sprintf("val-%06d", i))
				if err := s.Put(key, val); err != nil {
					t.Errorf("writer %d Put %d: %v", id, i, err)
					return
				}
			}
		}(w)
	}

	for r := range readers {
		go func(id int) {
			defer wg.Done()
			for i := range ops * 2 {
				key := []byte(fmt.Sprintf("preload-%04d", i%100))
				if _, err := s.Get(key); err != nil && err != ErrKeyNotFound {
					readErrors.Add(1)
				}
			}
		}(r)
	}
	wg.Wait()

	if readErrors.Load() > 0 {
		t.Errorf("readers encountered %d unexpected errors", readErrors.Load())
	}
	if s.Len() != int64(100+writers*ops) {
		t.Errorf("Len() = %d, want %d", s.Len(), 100+writers*ops)
	}
}

func TestSyncSkipList_ConcurrentMixed(t *testing.T) {
	const (
		workers = 16
		ops     = 300
	)
	s := NewSyncSkipList(32, 0.5)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := range workers {
		go func(id int) {
			defer wg.Done()
			for i := range ops {
				key := []byte(fmt.Sprintf("mix-w%02d-%06d", id, i))
				val := []byte(fmt.Sprintf("val-%06d", i))

				if err := s.Put(key, val); err != nil {
					t.Errorf("Put: %v", err)
					return
				}
				got, err := s.Get(key)
				if err != nil || !bytes.Equal(got, val) {
					t.Errorf("Get after Put: got %q err=%v", got, err)
					return
				}
				if i%3 == 0 {
					if err := s.Pop(key); err != nil {
						t.Errorf("Pop: %v", err)
						return
					}
					if _, err := s.Get(key); err != ErrKeyNotFound {
						t.Errorf("expected missing after pop, got err=%v", err)
						return
					}
				}
			}
		}(w)
	}
	wg.Wait()

	// Every worker inserts ops keys and pops ops/3 (integer division) of them.
	want := int64(workers * (ops - ops/3))
	if s.Len() != want {
		t.Errorf("Len() = %d, want %d", s.Len(), want)
	}
}

func TestSyncSkipList_ConcurrentPopSameKey(t *testing.T) {
	s := NewSyncSkipList(16, 0.5)
	key := []byte("shared")
	if err := s.Put(key, []byte("v")); err != nil {
		t.Fatalf("Put: %v", err)
	}

	const goroutines = 32
	var wg sync.WaitGroup
	var successes atomic.Int64
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			if err := s.Pop(key); err == nil {
				successes.Add(1)
			}
		}()
	}
	wg.Wait()

	if successes.Load() != 1 {
		t.Errorf("expected exactly 1 successful pop, got %d", successes.Load())
	}
	if s.Len() != 0 {
		t.Errorf("Len() = %d after concurrent pop, want 0", s.Len())
	}
}

// Benchmarks

func BenchmarkSyncSkipList_Push(b *testing.B) {
	b.ReportAllocs()
	keys := make([][]byte, b.N)
	for i := range keys {
		keys[i] = []byte(fmt.Sprintf("key-%08d", i))
	}
	s := NewSyncSkipList(32, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[i], keys[i])
	}
}

func BenchmarkSyncSkipList_Get(b *testing.B) {
	b.ReportAllocs()
	const n = 1000000
	s := NewSyncSkipList(32, 0.5)
	keys := make([][]byte, n)
	for i := 0; i < n; i++ {
		keys[i] = []byte(fmt.Sprintf("key-%08d", i))
		s.Put(keys[i], keys[i])
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Get(keys[i%n])
	}
}

func BenchmarkSyncSkipList_Pop(b *testing.B) {
	b.ReportAllocs()
	const n = 1000000
	keys := make([][]byte, n)
	for i := range keys {
		keys[i] = []byte(fmt.Sprintf("key-%08d", i))
	}
	s := NewSyncSkipList(32, 0.5)
	for j := range keys {
		s.Put(keys[j], keys[j])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := keys[i%n]
		s.Pop(k)
		s.Put(k, k)
	}
}

func BenchmarkSyncSkipList_Iteration(b *testing.B) {
	b.ReportAllocs()
	const n = 500000
	s := NewSyncSkipList(32, 0.5)
	for i := 0; i < n; i++ {
		k := []byte(fmt.Sprintf("key-%08d", i))
		s.Put(k, k)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		s.mu.RLock()
		for e := s.Header.nexts[0]; e != s.nilElement; e = e.nexts[0] {
			count++
		}
		s.mu.RUnlock()
		_ = count
	}
}

func BenchmarkSyncSkipList_PushGetMix(b *testing.B) {
	b.ReportAllocs()
	const preload = 1000000
	s := NewSyncSkipList(32, 0.5)
	for i := 0; i < preload; i++ {
		k := []byte(fmt.Sprintf("key-%08d", i))
		s.Put(k, k)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := []byte(fmt.Sprintf("key-%08d", i%preload))
		s.Put(k, k)
		_, _ = s.Get(k)
	}
}

func TestSyncSkipList_ActiveLevel(t *testing.T) {
	t.Run("EmptyIsNegativeOne", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		if s.level != -1 {
			t.Fatalf("level = %d, want -1", s.level)
		}
	})

	t.Run("BackToNegativeOneWhenEmpty", func(t *testing.T) {
		s := NewSyncSkipList(16, 0.5)
		_ = s.Put([]byte("a"), []byte("1"))
		if err := s.Pop([]byte("a")); err != nil {
			t.Fatal(err)
		}
		if s.level != -1 {
			t.Fatalf("level = %d after last pop, want -1", s.level)
		}
	})
}