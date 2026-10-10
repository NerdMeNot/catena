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
	"math/rand/v2"
	"slices"
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

//go:noinline
func handPipeline(xs []int) int {
	sum := 0
	for _, v := range xs {
		if v%2 != 0 {
			continue
		}
		v *= 3
		if v > 100 {
			sum += v
		}
	}
	return sum
}

func BenchmarkThreeWay_Pipeline_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSink = handPipeline(benchData)
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

// The hand-written loops live in functions of their own, as they would in
// a program. Written inline in the benchmark body they are measured under
// b.Loop's rule that nothing in the body may be optimised away, which
// keeps their accumulators in memory and makes a plain loop look several
// times slower than it is.
//
//go:noinline
func handSum(xs []int) int {
	sum := 0
	for _, v := range xs {
		sum += v
	}
	return sum
}

func BenchmarkThreeWay_Sum_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		benchSink = handSum(benchData)
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

// ---- The 1.5 operators against the loop each one replaces ----

// FoldedBy against the ordered grouped fold written by hand, and against
// FoldBy, whose map it trades for an index and two slices.
func BenchmarkFoldedBy_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		index := make(map[int]int)
		var keys, sums []int
		for _, v := range benchData {
			k := v % 64
			i, ok := index[k]
			if !ok {
				i = len(keys)
				index[k] = i
				keys = append(keys, k)
				sums = append(sums, 0)
			}
			sums[i] += v
		}
		n := 0
		for i, k := range keys {
			n += k + sums[i]
		}
		benchSink = n
	}
}

func BenchmarkFoldedBy_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for k, sum := range s.FoldedBy(
			func(v int) int { return v % 64 },
			func(int) int { return 0 },
			func(a, v int) int { return a + v },
		) {
			n += k + sum
		}
		benchSink = n
	}
}

func BenchmarkFoldedBy_FoldBy(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for k, sum := range s.FoldBy(
			func(v int) int { return v % 64 },
			func(int) int { return 0 },
			func(a, v int) int { return a + v },
		) {
			n += k + sum
		}
		benchSink = n
	}
}

func BenchmarkGroupedBy_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		index := make(map[int]int)
		var keys []int
		var groups [][]int
		for _, v := range benchData {
			k := v % 64
			i, ok := index[k]
			if !ok {
				i = len(keys)
				index[k] = i
				keys = append(keys, k)
				groups = append(groups, nil)
			}
			groups[i] = append(groups[i], v)
		}
		n := 0
		for i, k := range keys {
			n += k + len(groups[i])
		}
		benchSink = n
	}
}

func BenchmarkGroupedBy_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for k, g := range catena.GroupedBy(s, func(v int) int { return v % 64 }) {
			n += k + len(g)
		}
		benchSink = n
	}
}

func BenchmarkGroupedBy_GroupBy(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for k, g := range s.GroupBy(func(v int) int { return v % 64 }) {
			n += k + len(g)
		}
		benchSink = n
	}
}

func BenchmarkLeftJoin_Hand(b *testing.B) {
	left := benchData[:10_000]
	right := benchData[:1_000]
	b.ReportAllocs()
	for b.Loop() {
		idx := make(map[int][]int)
		for _, u := range right {
			idx[u%32] = append(idx[u%32], u)
		}
		n := 0
		for _, v := range left {
			us := idx[v%64] // half the left keys have no match
			if len(us) == 0 {
				n += v
			}
			for _, u := range us {
				n += v + u
			}
		}
		benchSink = n
	}
}

func BenchmarkLeftJoin_Catena(b *testing.B) {
	left := catena.FromSlice(benchData[:10_000])
	right := catena.FromSlice(benchData[:1_000])
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range left.LeftJoinBy(right,
			func(v int) int { return v % 64 },
			func(u int) int { return u % 32 },
			func(v, u int, _ bool) int { return v + u },
		) {
			n += v
		}
		benchSink = n
	}
}

func BenchmarkGroupJoin_Hand(b *testing.B) {
	left := benchData[:10_000]
	right := benchData[:1_000]
	b.ReportAllocs()
	for b.Loop() {
		idx := make(map[int][]int)
		for _, u := range right {
			idx[u%32] = append(idx[u%32], u)
		}
		n := 0
		for _, v := range left {
			n += v + len(idx[v%64])
		}
		benchSink = n
	}
}

func BenchmarkGroupJoin_Catena(b *testing.B) {
	left := catena.FromSlice(benchData[:10_000])
	right := catena.FromSlice(benchData[:1_000])
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range left.GroupJoinBy(right,
			func(v int) int { return v % 64 },
			func(u int) int { return u % 32 },
			func(v int, us []int) int { return v + len(us) },
		) {
			n += v
		}
		benchSink = n
	}
}

// Then is called once, when the pipeline is built: it must cost nothing
// per element.
func BenchmarkThen_Without(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		benchSink = catena.Sum(catena.Distinct(s.Map(func(v int) int { return v % 1024 })))
	}
}

func BenchmarkThen_With(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		benchSink = s.Map(func(v int) int { return v % 1024 }).Then(catena.Distinct).Then(catena.Sum)
	}
}

//go:noinline
func handIsSorted(xs []int) bool {
	for i := 1; i < len(xs); i++ {
		if xs[i-1] > xs[i] {
			return false
		}
	}
	return true
}

func BenchmarkIsSorted_Hand(b *testing.B) {
	sorted := slices.Sorted(slices.Values(benchData))
	b.ReportAllocs()
	for b.Loop() {
		if handIsSorted(sorted) {
			benchSink++
		}
	}
}

func BenchmarkIsSorted_Catena(b *testing.B) {
	s := catena.FromSlice(slices.Sorted(slices.Values(benchData)))
	b.ReportAllocs()
	for b.Loop() {
		if catena.IsSorted(s) {
			benchSink++
		}
	}
}

func BenchmarkDuplicates_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		reported := make(map[int]bool)
		n := 0
		for _, v := range benchData {
			k := v % 4096
			done, seen := reported[k]
			if !seen {
				reported[k] = false
			} else if !done {
				reported[k] = true
				n += k
			}
		}
		benchSink = n
	}
}

func BenchmarkDuplicates_Catena(b *testing.B) {
	s := catena.FromSlice(benchData).Map(func(v int) int { return v % 4096 })
	b.ReportAllocs()
	for b.Loop() {
		benchSink = catena.Sum(catena.Duplicates(s))
	}
}

// Sample against the two things it replaces: collecting everything to
// shuffle it, and the textbook reservoir that draws a random number per
// element.
func BenchmarkSample_CollectAndShuffle(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	b.ReportAllocs()
	for b.Loop() {
		all := slices.Clone(benchData)
		rng.Shuffle(len(all), func(i, j int) { all[i], all[j] = all[j], all[i] })
		benchSink = all[0] + len(all[:10])
	}
}

func BenchmarkSample_HandReservoir(b *testing.B) {
	rng := rand.New(rand.NewPCG(1, 2))
	b.ReportAllocs()
	for b.Loop() {
		res := make([]int, 0, 10)
		for i, v := range benchData {
			if len(res) < 10 {
				res = append(res, v)
			} else if j := rng.IntN(i + 1); j < 10 {
				res[j] = v
			}
		}
		benchSink = res[0]
	}
}

func BenchmarkSample_Catena(b *testing.B) {
	s := catena.FromSlice(benchData)
	rng := rand.New(rand.NewPCG(1, 2))
	b.ReportAllocs()
	for b.Loop() {
		benchSink = s.Sample(10, rng)[0]
	}
}

// ---- Raw-closure twins of the 1.5 operators: the equal-terms baseline ----
// Each is the operator written the way a person would write it on top of
// iter.Seq — range over the source, callbacks for the varying parts —
// which is what the library's version must not be slower than.

func rawLeftJoin(left, right iter.Seq[int], lk, rk func(int) int, combine func(int, int, bool) int) iter.Seq[int] {
	return func(yield func(int) bool) {
		idx := make(map[int][]int)
		for u := range right {
			idx[rk(u)] = append(idx[rk(u)], u)
		}
		for v := range left {
			us := idx[lk(v)]
			if len(us) == 0 {
				if !yield(combine(v, 0, false)) {
					return
				}
				continue
			}
			for _, u := range us {
				if !yield(combine(v, u, true)) {
					return
				}
			}
		}
	}
}

func BenchmarkLeftJoin_Raw(b *testing.B) {
	left, right := rawSlice(benchData[:10_000]), rawSlice(benchData[:1_000])
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range rawLeftJoin(left, right,
			func(v int) int { return v % 64 },
			func(u int) int { return u % 32 },
			func(v, u int, _ bool) int { return v + u },
		) {
			n += v
		}
		benchSink = n
	}
}

func rawGroupJoin(left, right iter.Seq[int], lk, rk func(int) int, combine func(int, []int) int) iter.Seq[int] {
	return func(yield func(int) bool) {
		idx := make(map[int][]int)
		for u := range right {
			idx[rk(u)] = append(idx[rk(u)], u)
		}
		for v := range left {
			if !yield(combine(v, idx[lk(v)])) {
				return
			}
		}
	}
}

func BenchmarkGroupJoin_Raw(b *testing.B) {
	left, right := rawSlice(benchData[:10_000]), rawSlice(benchData[:1_000])
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range rawGroupJoin(left, right,
			func(v int) int { return v % 64 },
			func(u int) int { return u % 32 },
			func(v int, us []int) int { return v + len(us) },
		) {
			n += v
		}
		benchSink = n
	}
}

func rawFoldedBy(s iter.Seq[int], key func(int) int, init func(int) int, f func(int, int) int) iter.Seq2[int, int] {
	return func(yield func(int, int) bool) {
		index := make(map[int]int)
		var keys, accs []int
		for v := range s {
			k := key(v)
			i, ok := index[k]
			if !ok {
				i = len(keys)
				index[k] = i
				keys = append(keys, k)
				accs = append(accs, init(k))
			}
			accs[i] = f(accs[i], v)
		}
		for i, k := range keys {
			if !yield(k, accs[i]) {
				return
			}
		}
	}
}

func BenchmarkFoldedBy_Raw(b *testing.B) {
	s := rawSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for k, sum := range rawFoldedBy(s,
			func(v int) int { return v % 64 },
			func(int) int { return 0 },
			func(a, v int) int { return a + v },
		) {
			n += k + sum
		}
		benchSink = n
	}
}

func rawGroupedBy(s iter.Seq[int], key func(int) int) iter.Seq2[int, []int] {
	return func(yield func(int, []int) bool) {
		index := make(map[int]int)
		var keys []int
		var groups [][]int
		for v := range s {
			k := key(v)
			i, ok := index[k]
			if !ok {
				i = len(keys)
				index[k] = i
				keys = append(keys, k)
				groups = append(groups, nil)
			}
			groups[i] = append(groups[i], v)
		}
		for i, k := range keys {
			if !yield(k, groups[i]) {
				return
			}
		}
	}
}

func BenchmarkGroupedBy_Raw(b *testing.B) {
	s := rawSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for k, g := range rawGroupedBy(s, func(v int) int { return v % 64 }) {
			n += k + len(g)
		}
		benchSink = n
	}
}

func rawDuplicates(s iter.Seq[int]) iter.Seq[int] {
	return func(yield func(int) bool) {
		reported := make(map[int]bool)
		for v := range s {
			done, seen := reported[v]
			if !seen {
				reported[v] = false
				continue
			}
			if done {
				continue
			}
			reported[v] = true
			if !yield(v) {
				return
			}
		}
	}
}

func BenchmarkDuplicates_Raw(b *testing.B) {
	s := rawMap(rawSlice(benchData), func(v int) int { return v % 4096 })
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range rawDuplicates(s) {
			n += v
		}
		benchSink = n
	}
}

func rawIsSorted(s iter.Seq[int]) bool {
	prev, have := 0, false
	for v := range s {
		if have && prev > v {
			return false
		}
		prev, have = v, true
	}
	return true
}

func BenchmarkIsSorted_Raw(b *testing.B) {
	s := rawSlice(slices.Sorted(slices.Values(benchData)))
	b.ReportAllocs()
	for b.Loop() {
		if rawIsSorted(s) {
			benchSink++
		}
	}
}
