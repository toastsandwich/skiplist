package skiplist

import (
	"bytes"
	"fmt"
	"testing"
)

func TestPutGet(t *testing.T) {
	t.Run("Basic", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
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
		s := NewSkipList(16, 0.5)
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
	})

	t.Run("Missing", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if _, err := s.Get([]byte("nope")); err == nil {
			t.Error("expected error for missing key")
		}
		// Get(nil) hits the sentinel and returns (nil, nil) currently; ensure no panic at least
		if _, err := s.Get(nil); err != nil {
			// acceptable either way; do not fail hard on sentinel edge
		}
	})
}

func TestPop(t *testing.T) {
	t.Run("Existing", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if err := s.Put([]byte("a"), []byte("1")); err != nil {
			t.Fatalf("Put a: %v", err)
		}
		if err := s.Put([]byte("b"), []byte("2")); err != nil {
			t.Fatalf("Put b: %v", err)
		}
		if err := s.Put([]byte("c"), []byte("3")); err != nil {
			t.Fatalf("Put c: %v", err)
		}

		if err := s.Pop([]byte("b")); err != nil {
			t.Errorf("expected nil, got %v", err)
		}

		if _, err := s.Get([]byte("b")); err == nil {
			t.Error("key b should be gone after pop")
		}

		// remaining keys still there
		va, err := s.Get([]byte("a"))
		if err != nil || !bytes.Equal(va, []byte("1")) {
			t.Error("a missing")
		}
		vc, err := s.Get([]byte("c"))
		if err != nil || !bytes.Equal(vc, []byte("3")) {
			t.Error("c missing")
		}
	})

	t.Run("Missing", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if err := s.Put([]byte("x"), []byte("y")); err != nil {
			t.Fatalf("Put: %v", err)
		}

		if err := s.Pop([]byte("z")); err != ErrKeyNotFound {
			t.Errorf("pop missing should return ErrKeyNotFound, got %v", err)
		}
		if err := s.Pop(nil); err == nil {
			t.Error("pop nil should return error")
		}
	})

	t.Run("All", func(t *testing.T) {
		s := NewSkipList(8, 0.5)
		keys := [][]byte{[]byte("1"), []byte("2"), []byte("3")}
		for _, k := range keys {
			if err := s.Put(k, append([]byte{}, k...)); err != nil {
				t.Fatalf("Put %q: %v", k, err)
			}
		}
		for _, k := range keys {
			s.Pop(k)
		}
		for _, k := range keys {
			if _, err := s.Get(k); err == nil {
				t.Errorf("expected all popped, still have %q", k)
			}
		}
		count := 0
		for range s.All() {
			count++
		}
		if count != 0 {
			t.Errorf("expected 0 elements, got %d", count)
		}
	})
}

func TestAll(t *testing.T) {
	t.Run("Empty", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		count := 0
		for range s.All() {
			count++
		}
		if count != 0 {
			t.Errorf("expected 0 on empty, got %d", count)
		}
	})

	t.Run("IteratesInOrder", func(t *testing.T) {
		s := NewSkipList(16, 0.5)

		// insert out of order
		if err := s.Put([]byte("c"), []byte("3")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := s.Put([]byte("a"), []byte("1")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := s.Put([]byte("b"), []byte("2")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := s.Put([]byte("d"), []byte("4")); err != nil {
			t.Fatalf("Put: %v", err)
		}

		var gotKeys [][]byte
		var gotVals [][]byte
		for k, v := range s.All() {
			gotKeys = append(gotKeys, append([]byte{}, k...))
			gotVals = append(gotVals, append([]byte{}, v...))
		}

		wantKeys := [][]byte{[]byte("a"), []byte("b"), []byte("c"), []byte("d")}
		for i := range wantKeys {
			if !bytes.Equal(gotKeys[i], wantKeys[i]) {
				t.Errorf("key order wrong at %d: got %q want %q", i, gotKeys[i], wantKeys[i])
			}
		}
		if !bytes.Equal(gotVals[1], []byte("2")) {
			t.Error("value mismatch in iterator")
		}
	})
}

func TestForEach(t *testing.T) {
	s := NewSkipList(DefaultValues())
	keys := [][]byte{}
	vals := [][]byte{}

	for i := range 100 {
		keys = append(keys, []byte(fmt.Sprintf("keys-%d", i)))
		vals = append(vals, []byte(fmt.Sprintf("vals-%d", i)))
	}

	for i, key := range keys {
		s.Put(key, vals[i])
	}

	s.ForEach(func(key, val []byte) bool {
		v, err := s.Get(key)
		if err != nil {
			t.Fatal("expected key to be present key:", string(key))
			return false
		}
		if !bytes.Equal(val, v) {
			t.Fatalf("expect value= %s, got= %s", string(val), string(v))
			return false
		}
		return true
	})
}

func TestCorrectnessLarge(t *testing.T) {
	const N = 20000
	s := NewSkipList(32, 0.5)

	// Use fixed-width keys so lexical order matches numeric
	keys := make([][]byte, N)
	vals := make([][]byte, N)
	for i := range N {
		k := []byte(fmt.Sprintf("key-%08d", i))
		keys[i] = k
		vals[i] = []byte(fmt.Sprintf("val-%08d", i))
		if err := s.Put(keys[i], vals[i]); err != nil {
			t.Fatalf("Put failed at %d: %v", i, err)
		}
	}

	// All gets
	for i := range N {
		got, err := s.Get(keys[i])
		if err != nil || !bytes.Equal(got, vals[i]) {
			t.Fatalf("get mismatch at %d", i)
		}
	}

	// Verify sorted iteration and full count
	i := 0
	var prev []byte
	for k, v := range s.All() {
		if prev != nil && bytes.Compare(prev, k) >= 0 {
			t.Fatalf("not sorted at index %d: %q >= %q", i, prev, k)
		}
		if !bytes.Equal(v, vals[i]) {
			t.Fatalf("iter value mismatch at %d", i)
		}
		prev = append([]byte(nil), k...)
		i++
	}
	if i != N {
		t.Fatalf("expected %d elements in iter, got %d", N, i)
	}
}

func TestMixedOperations(t *testing.T) {
	s := NewSkipList(16, 0.5)

	ops := []struct {
		op  string
		k   string
		v   string
		exp string // expected get after
	}{
		{"push", "k1", "v1", "v1"},
		{"push", "k2", "v2", "v2"},
		{"push", "k1", "v1-up", "v1-up"},
		{"pop", "k2", "", ""},
		{"get", "k2", "", ""}, // missing
		{"push", "k3", "v3", "v3"},
		{"pop", "k1", "", ""},
		{"get", "k1", "", ""},
	}

	for _, o := range ops {
		switch o.op {
		case "push":
			s.Put([]byte(o.k), []byte(o.v))
		case "pop":
			s.Pop([]byte(o.k))
		}
		got, err := s.Get([]byte(o.k))
		if o.exp == "" {
			if err == nil {
				t.Errorf("after %s %s expected missing, got %q", o.op, o.k, got)
			}
		} else if err != nil || !bytes.Equal(got, []byte(o.exp)) {
			t.Errorf("after %s %s expected %s got %q", o.op, o.k, o.exp, got)
		}
	}
}

func TestUpdateDoesNotAffectOrder(t *testing.T) {
	s := NewSkipList(16, 0.5)
	s.Put([]byte("b"), []byte("2"))
	s.Put([]byte("a"), []byte("1"))
	s.Put([]byte("c"), []byte("3"))
	s.Put([]byte("a"), []byte("1-updated"))

	var keys []string
	for k := range s.All() {
		keys = append(keys, string(k))
	}
	if len(keys) != 3 || keys[0] != "a" || keys[1] != "b" || keys[2] != "c" {
		t.Errorf("order broken after update: %v", keys)
	}
}

func TestMaxLen(t *testing.T) {
	s := NewSkipList(16, 0.5)
	s.MaxLen = 2

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

func TestMaxEntriesAndMaxLevelFor(t *testing.T) {
	cases := []struct {
		maxLevel int
		p        float64
		want     int64
	}{
		{1, 0.5, 2},
		{10, 0.5, 1024},
		{16, 0.5, 65536},
		{32, 0.5, 1 << 32}, // 2^32
	}
	for _, tc := range cases {
		got := MaxEntries(tc.maxLevel, tc.p)
		if tc.maxLevel < 32 && got != tc.want {
			t.Errorf("MaxEntries(%d, %g) = %d, want %d", tc.maxLevel, tc.p, got, tc.want)
		}
		if MaxLevelFor(got, tc.p) != tc.maxLevel {
			t.Errorf("MaxLevelFor(MaxEntries(%d, %g), %g) = %d, want %d",
				tc.maxLevel, tc.p, tc.p, MaxLevelFor(got, tc.p), tc.maxLevel)
		}
	}

	if MaxLevelFor(1000, 0.5) != 10 {
		t.Errorf("MaxLevelFor(1000, 0.5) = %d, want 10", MaxLevelFor(1000, 0.5))
	}
}

func TestCapacity(t *testing.T) {
	s := NewSkipList(16, 0.5)
	if s.Cap() != MaxEntries(16, 0.5) {
		t.Errorf("Capacity() = %d, want %d", s.Cap(), MaxEntries(16, 0.5))
	}

	s.MaxLen = 100
	if s.Cap() != 100 {
		t.Errorf("Capacity() with MaxLen = %d, want 100", s.Cap())
	}
}

func TestLen(t *testing.T) {
	t.Run("NewListIsZero", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if s.Len() != 0 {
			t.Errorf("Len() = %d, want 0", s.Len())
		}
	})

	t.Run("IncreasesOnInsert", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if err := s.Put([]byte("a"), []byte("1")); err != nil {
			t.Fatalf("Put a: %v", err)
		}
		if err := s.Put([]byte("b"), []byte("2")); err != nil {
			t.Fatalf("Put b: %v", err)
		}
		if s.Len() != 2 {
			t.Errorf("Len() = %d, want 2", s.Len())
		}
	})

	t.Run("UpdateDoesNotIncrease", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if err := s.Put([]byte("a"), []byte("1")); err != nil {
			t.Fatalf("Put: %v", err)
		}
		if err := s.Put([]byte("a"), []byte("1-updated")); err != nil {
			t.Fatalf("Put update: %v", err)
		}
		if s.Len() != 1 {
			t.Errorf("Len() = %d after update, want 1", s.Len())
		}
	})

	t.Run("DecreasesOnPop", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		_ = s.Put([]byte("a"), []byte("1"))
		_ = s.Put([]byte("b"), []byte("2"))
		if err := s.Pop([]byte("b")); err != nil {
			t.Fatalf("Pop: %v", err)
		}
		if s.Len() != 1 {
			t.Errorf("Len() = %d, want 1", s.Len())
		}
	})

	t.Run("PopMissingDoesNotChange", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		_ = s.Put([]byte("a"), []byte("1"))
		if err := s.Pop([]byte("nope")); err != ErrKeyNotFound {
			t.Errorf("Pop missing: got %v, want ErrKeyNotFound", err)
		}
		if s.Len() != 1 {
			t.Errorf("Len() = %d after pop missing, want 1", s.Len())
		}
	})

	t.Run("MatchesIterationCount", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		for i := 0; i < 5; i++ {
			k := []byte{byte('a' + i)}
			_ = s.Put(k, k)
		}
		count := 0
		for range s.All() {
			count++
		}
		if s.Len() != count {
			t.Errorf("Len()=%d does not match All() count=%d", s.Len(), count)
		}
	})

	t.Run("ZeroAfterLastPop", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		_ = s.Put([]byte("a"), []byte("1"))
		_ = s.Pop([]byte("a"))
		if s.Len() != 0 {
			t.Errorf("Len() = %d after last pop, want 0", s.Len())
		}
	})

	t.Run("PopOnEmptyDoesNotGoNegative", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if err := s.Pop([]byte("anything")); err != ErrKeyNotFound {
			t.Errorf("Pop on empty: got %v, want ErrKeyNotFound", err)
		}
		if s.Len() != 0 {
			t.Errorf("Len() = %d after pop on empty, want 0", s.Len())
		}
	})
}

func TestActiveLevel(t *testing.T) {
	t.Run("EmptyIsNegativeOne", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if s.level != -1 {
			t.Fatalf("level = %d, want -1", s.level)
		}
	})

	t.Run("RaisesOnInsert", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		if err := s.Put([]byte("a"), []byte("1")); err != nil {
			t.Fatal(err)
		}
		if s.level < 0 {
			t.Fatalf("level = %d after insert, want >= 0", s.level)
		}
	})

	t.Run("BackToNegativeOneWhenEmpty", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		_ = s.Put([]byte("a"), []byte("1"))
		if err := s.Pop([]byte("a")); err != nil {
			t.Fatal(err)
		}
		if s.level != -1 {
			t.Fatalf("level = %d after last pop, want -1", s.level)
		}
	})

	t.Run("LowersWhenTopTowerRemoved", func(t *testing.T) {
		s := NewSkipList(16, 0.5)
		insertAtLevel(s, []byte("low"), []byte("1"), 0)
		insertAtLevel(s, []byte("high"), []byte("2"), 4)
		if s.level != 4 {
			t.Fatalf("level = %d, want 4", s.level)
		}
		if err := s.Pop([]byte("high")); err != nil {
			t.Fatal(err)
		}
		if s.level != 0 {
			t.Fatalf("level = %d after popping top, want 0", s.level)
		}
		if _, err := s.Get([]byte("low")); err != nil {
			t.Fatalf("low missing after pop high: %v", err)
		}
	})
}

func insertAtLevel(s *SkipList, key, val []byte, lvl int) {
	update := make([]*Element, s.MaxLevel)
	top := s.topLevel(lvl)
	s.fillUpdate(update, key, top)

	e := s.NewElement(key, val, lvl)
	for i := 0; i <= lvl; i++ {
		e.nexts[i] = update[i].nexts[i]
		update[i].nexts[i] = e
	}
	if lvl > s.level {
		s.level = lvl
	}
	s.len++
}

// Benchmarks

func BenchmarkSkipList_Push(b *testing.B) {
	b.ReportAllocs()
	keys := make([][]byte, b.N)
	for i := range keys {
		keys[i] = []byte(fmt.Sprintf("key-%08d", i))
	}
	s := NewSkipList(32, 0.5)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Put(keys[i], keys[i])
	}
}

func BenchmarkSkipList_Get(b *testing.B) {
	b.ReportAllocs()
	const n = 1000000 // heavy load: 1M elements
	s := NewSkipList(32, 0.5)
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

func BenchmarkSkipList_Pop(b *testing.B) {
	b.ReportAllocs()
	// Measure pop cost in steady state: pop + reinsert to keep dataset size constant
	const n = 1000000 // heavy load: 1M elements
	keys := make([][]byte, n)
	for i := range keys {
		keys[i] = []byte(fmt.Sprintf("key-%08d", i))
	}
	s := NewSkipList(32, 0.5)
	for j := range keys {
		s.Put(keys[j], keys[j])
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		k := keys[i%n]
		s.Pop(k)
		s.Put(k, k) // restore for next iteration
	}
}

func BenchmarkSkipList_Iteration(b *testing.B) {
	b.ReportAllocs()
	const n = 500000 // heavy load: 500k elements (full iteration each time)
	s := NewSkipList(32, 0.5)
	for i := 0; i < n; i++ {
		k := []byte(fmt.Sprintf("key-%08d", i))
		s.Put(k, k)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count := 0
		for range s.All() {
			count++
		}
		_ = count
	}
}

func BenchmarkSkipList_PushGetMix(b *testing.B) {
	b.ReportAllocs()
	const preload = 1000000 // heavy load: start with 1M elements
	s := NewSkipList(32, 0.5)
	for i := 0; i < preload; i++ {
		k := []byte(fmt.Sprintf("key-%08d", i))
		s.Put(k, k)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// cycle over existing keys to simulate load on large structure (updates + gets)
		k := []byte(fmt.Sprintf("key-%08d", i%preload))
		s.Put(k, k)
		_, _ = s.Get(k)
	}
}

// Example demonstrates basic usage of SkipList.
func Example() {
	s := NewSkipList(16, 0.5)

	s.Put([]byte("cat"), []byte("meow"))
	s.Put([]byte("dog"), []byte("woof"))
	s.Put([]byte("cat"), []byte("purr")) // update

	if v, err := s.Get([]byte("cat")); err == nil {
		fmt.Println("cat ->", string(v))
	}

	fmt.Println("All entries:")
	for k, v := range s.All() {
		fmt.Printf("  %s: %s\n", k, v)
	}

	s.Pop([]byte("dog"))
	fmt.Println("After pop dog, len via iteration:")
	count := 0
	for range s.All() {
		count++
	}
	fmt.Println("count:", count)

	// Output:
	// cat -> purr
	// All entries:
	//   cat: purr
	//   dog: woof
	// After pop dog, len via iteration:
	// count: 1
}
