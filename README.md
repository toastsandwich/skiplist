# go-skip-list

A clean, efficient [skip list](https://en.wikipedia.org/wiki/Skip_list) implementation in Go.

This package provides a probabilistic sorted key-value store with **expected O(log n)** performance for search, insertion, and deletion.

## Features

- Keys and values are `[]byte`
- Keys are maintained in sorted order (`bytes.Compare`)
- `Push` acts as upsert (insert or update)
- Zero-allocation `Get` in the common case
- Iterator support via `iter.Seq2` (`All()`)
- Configurable max level and promotion probability

## Installation

```bash
go get github.com/toastsandwich/skiplist
```

## Quick Start

```go
package main

import (
	"fmt"
	"github.com/toastsandwich/skiplist"
)

func main() {
	s := skiplist.NewSkipList(32, 0.5)

	s.Push([]byte("user:1001"), []byte("Alice"))
	s.Push([]byte("user:1002"), []byte("Bob"))
	s.Push([]byte("user:1001"), []byte("Alice Smith")) // update

	fmt.Println(string(s.Get([]byte("user:1001")))) // Alice Smith

	for k, v := range s.All() {
		fmt.Printf("%s => %s\n", k, v)
	}

	s.Pop([]byte("user:1002"))
}
```

## Documentation

Run `go doc` or visit the generated documentation:

```bash
go doc github.com/toastsandwich/skiplist
# or
go doc -all .
```

## How Skip Lists Work

A skip list is a layered linked list that allows fast search by "skipping" over many elements.

### Structure

```
Level 3:  HEAD -----------------------------------------------> NIL
Level 2:  HEAD --------------------> [30] --------------------> NIL
Level 1:  HEAD --> [10] -----------> [30] -----------> [50] --> NIL
Level 0:  HEAD --> [10] --> [20] --> [30] --> [40] --> [50] --> NIL
```

- Every element lives on **level 0**.
- When inserting, we randomly decide how many levels the element should occupy.
- Higher levels contain fewer elements → they act as "express lanes".
- Search starts at the highest level and moves right, then drops down levels as needed.

### Insertion Height

The height of a new node is chosen using a geometric distribution:

```go
for l < maxLevel-1 && rand.Float64() < p {
    l++
}
```

With the default `p = 0.5`, roughly:

- 50% of nodes appear only on level 0
- 25% reach level 1
- 12.5% reach level 2
- ...and so on

This gives expected **O(log n)** search time with high probability.

### Search Walk

To find a key:

1. Start at the top level from the header.
2. Move right while the next key is **less than** the search key.
3. When you can't move right, drop one level.
4. Repeat until level 0.
5. Check if the node at level 0 matches.

### Why `[]byte`?

Using byte slices makes the structure very flexible (you can encode strings, integers, composite keys, etc.). Ordering is defined by Go's `bytes.Compare`.

## API

```go
func NewSkipList(maxlevel int, p float64) *SkipList

func (s *SkipList) Push(key, val []byte)
func (s *SkipList) Get(key []byte) []byte
func (s *SkipList) Pop(key []byte) []byte
func (s *SkipList) All() iter.Seq2[[]byte, []byte]
```

### NewSkipList

- `maxlevel`: maximum height of the skip list (defaults to 32 if ≤ 0)
- `p`: promotion probability (defaults to 0.5 if ≤ 0)

Recommended call:

```go
s := skiplist.NewSkipList(32, 0.5)
```

### Push / Get / Pop

- `Push` inserts or updates a key.
- `Get` returns `nil` for missing keys.
- `Pop` removes a key and returns its old value (or `nil`).

### Iteration

```go
for key, value := range s.All() {
    // keys are yielded in ascending order
}
```

The iterator reflects the state at the time of iteration. Modifying the list during iteration is not safe.

## Examples

### Basic Operations

```go
s := skiplist.NewSkipList(16, 0.5)

s.Push([]byte("a"), []byte("1"))
s.Push([]byte("b"), []byte("2"))
s.Push([]byte("a"), []byte("updated"))   // overwrite

v := s.Get([]byte("a"))                   // []byte("updated")
old := s.Pop([]byte("b"))                 // []byte("2")
```

## Benchmarks

Benchmarks were run on:

- CPU: Intel Core i7-11800H (2.3 GHz)
- OS: Linux
- Go: 1.26.4

```bash
go test -bench=. -benchmem -count=3 -benchtime=2s
```

### Results

| Benchmark                  | ns/op    | B/op | allocs/op | Notes                               |
|---------------------------|----------|------|-----------|-------------------------------------|
| `Push`                    | 450      | 608  | 3         | Insert new key                      |
| `Get`                     | 181      | 0    | 0         | Zero-allocation read path           |
| `Pop` (+ reinsert)        | 688      | 864  | 4         | Steady-state delete + restore       |
| `Iteration` (10k elems)   | ~89 000  | 0    | 0         | Full scan via `All()` (~89 µs)      |
| `PushGetMix`              | 790      | 648  | 5         | Combined insert + lookup workload   |

> **Note**: The `Pop` benchmark measures a pop followed by a re-insert to keep the dataset size stable across iterations. All numbers are averages across multiple runs.

### Key Observations
- `Get` performs **zero allocations** on hits.
- Full iteration over 10,000 elements completes in ~89 µs.
- The structure is very cache-friendly for sequential iteration.

## Limitations

- **Not thread-safe** — external synchronization is required for concurrent use.
- `Get` cannot distinguish between a missing key and a key whose value is `nil`/`[]byte{}`.
- `currMaxLevel` is tracked internally but currently unused (the structure always uses the full configured `MaxLevel`).
- Designed primarily for in-memory use cases.

## License

This project is available under the terms of your choice (no license file included yet).

---

*Contributions and improvements are welcome!*
