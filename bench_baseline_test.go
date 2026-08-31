package catena_test

// Three-way baselines separating the cost of the LANGUAGE (range-over-func
// closure calls) from the cost of the LIBRARY. For each shape:
//
//   Hand    — plain loop, no iterators: the absolute floor.
//   Raw     — the same pipeline hand-built from bare iter.Seq closures:
//             the floor of the rangefunc mechanism itself.
//   Catena  — the library.
//
// The library's contract: Catena ≈ Raw within noise. Any gap between them
// is catena-added overhead and is a bug to fix; the Raw−Hand gap is Go's.

import (
	"iter"
	"testing"

	"github.com/NerdMeNot/catena"
)

func rawFilter(s iter.Seq[int], pred func(int) bool) iter.Seq[int] {
	return func(yield func(int) bool) {
		for v := range s {
			if pred(v) && !yield(v) {
				return
			}
		}
	}
}

func rawMap(s iter.Seq[int], f func(int) int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func rawSlice(xs []int) iter.Seq[int] {
	return func(yield func(int) bool) {
		for _, v := range xs {
			if !yield(v) {
				return
			}
		}
	}
}

var (
	benchEven   = func(v int) bool { return v%2 == 0 }
	benchTriple = func(v int) int { return v * 3 }
	benchGt100  = func(v int) bool { return v > 100 }
)

// ---- 4-stage pipeline: filter → map → filter → sum ----

func BenchmarkThreeWay_Pipeline_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for _, v := range benchData {
			if v%2 != 0 {
				continue
			}
			v *= 3
			if v > 100 {
				sum += v
			}
		}
		benchSink = sum
	}
}

func BenchmarkThreeWay_Pipeline_Raw(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for v := range rawFilter(rawMap(rawFilter(rawSlice(benchData), benchEven), benchTriple), benchGt100) {
			sum += v
		}
		benchSink = sum
	}
}

func BenchmarkThreeWay_Pipeline_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for v := range s.Filter(benchEven).Map(benchTriple).Filter(benchGt100) {
			sum += v
		}
		benchSink = sum
	}
}

// ---- single stage: map → drain ----

func BenchmarkThreeWay_Map_Raw(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for v := range rawMap(rawSlice(benchData), benchTriple) {
			sum += v
		}
		benchSink = sum
	}
}

func BenchmarkThreeWay_Map_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for v := range s.Map(benchTriple) {
			sum += v
		}
		benchSink = sum
	}
}

// ---- Sum: does delegation through SumOf(Self) cost per element? ----

func BenchmarkThreeWay_Sum_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for _, v := range benchData {
			sum += v
		}
		benchSink = sum
	}
}

func BenchmarkThreeWay_Sum_Raw(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for v := range rawSlice(benchData) {
			sum += v
		}
		benchSink = sum
	}
}

func BenchmarkThreeWay_Sum_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		benchSink = catena.Sum(s)
	}
}

// ---- Distinct ----

func rawDistinct(s iter.Seq[int]) iter.Seq[int] {
	return func(yield func(int) bool) {
		seen := make(map[int]struct{})
		for v := range s {
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			if !yield(v) {
				return
			}
		}
	}
}

func BenchmarkThreeWay_Distinct_Raw(b *testing.B) {
	// structurally identical to the catena version: a distinct stage,
	// then a counting range over it
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for range rawDistinct(rawSlice(benchData)) {
			n++
		}
		benchSink = n
	}
}

func BenchmarkThreeWay_Distinct_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		benchSink = catena.Distinct(s).Count()
	}
}

// ---- Contains (worst case: absent) ----

func BenchmarkThreeWay_Contains_Raw(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		found := false
		for v := range rawSlice(benchData) {
			if v == -1 {
				found = true
				break
			}
		}
		if found {
			benchSink++
		}
	}
}

func BenchmarkThreeWay_Contains_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		if catena.Contains(s, -1) {
			benchSink++
		}
	}
}

// ---- SortedBy with a non-trivial selector: O(n log n) vs O(n) sel calls ----

func BenchmarkSortedBy_SelectorCalls(b *testing.B) {
	s := catena.FromSlice(benchData[:10_000])
	calls := 0
	sel := func(v int) int { calls++; return v % 97 }
	b.ReportAllocs()
	for b.Loop() {
		calls = 0
		benchSink = s.SortedBy(sel).Count()
	}
	b.ReportMetric(float64(calls), "selcalls/op")
}

// ---- Zip / Equal: the iter.Pull coroutine cost, per element ----

func BenchmarkZip_PullCost(b *testing.B) {
	a := catena.FromSlice(benchData[:10_000])
	c := catena.FromSlice(benchData[:10_000])
	b.ReportAllocs()
	for b.Loop() {
		benchSink = a.Zip(c).MapTo(func(x, y int) int { return x + y }).Count()
	}
}

// ---- JoinBy vs a hand-written hash join (the graduation criterion) ----

func BenchmarkJoin_Hand(b *testing.B) {
	left := benchData[:10_000]
	right := benchData[:1_000]
	b.ReportAllocs()
	for b.Loop() {
		idx := make(map[int][]int)
		for _, u := range right {
			idx[u%16] = append(idx[u%16], u)
		}
		n := 0
		for _, v := range left {
			for _, u := range idx[v%16] {
				n += v + u
			}
		}
		benchSink = n
	}
}

func BenchmarkJoin_Catena(b *testing.B) {
	left := catena.FromSlice(benchData[:10_000])
	right := catena.FromSlice(benchData[:1_000])
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range left.JoinBy(right,
			func(v int) int { return v % 16 },
			func(u int) int { return u % 16 },
			func(v, u int) int { return v + u },
		) {
			n += v
		}
		benchSink = n
	}
}

// ---- Isolating the residual gap: is it catena, or Go generics? ----
// rawMapG/rawDistinctG are byte-for-byte the raw implementations with a
// type parameter added. If they cost the same as catena (and more than
// their concrete twins), the residual single-stage gap is the generics
// dictionary/inliner, not the library.

func rawMapG[T any](s iter.Seq[T], f func(T) T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for v := range s {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func rawSliceG[T any](xs []T) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range xs {
			if !yield(v) {
				return
			}
		}
	}
}

func rawDistinctG[T comparable](s iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		seen := make(map[T]struct{})
		for v := range s {
			if _, dup := seen[v]; dup {
				continue
			}
			seen[v] = struct{}{}
			if !yield(v) {
				return
			}
		}
	}
}

func BenchmarkThreeWay_Map_RawGeneric(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sum := 0
		for v := range rawMapG(rawSliceG(benchData), benchTriple) {
			sum += v
		}
		benchSink = sum
	}
}

func BenchmarkThreeWay_Distinct_RawGeneric(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for range rawDistinctG(rawSliceG(benchData)) {
			n++
		}
		benchSink = n
	}
}
