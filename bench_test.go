package catena_test

// Benchmarks backing the performance guide: the honest numbers against
// hand-written loops, the generic-method-vs-package-function question,
// and the fused-operator wins. Run: go test -bench . -benchmem

import (
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

var benchData = func() []int {
	xs := make([]int, 100_000)
	for i := range xs {
		xs[i] = (i * 7919) % 1000
	}
	return xs
}()

var benchSink int

func BenchmarkPipeline4Stage(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		benchSink = s.
			Filter(func(v int) bool { return v%2 == 0 }).
			Map(func(v int) int { return v * 3 }).
			Filter(func(v int) bool { return v > 100 }).
			SumOf(catena.Self[int])
	}
}

func BenchmarkPipeline4StageHandLoop(b *testing.B) {
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

// mapFunc is the package-function twin of Seq.Map, for measuring whether
// generic METHODS (new in Go 1.27) carry a dictionary-call penalty over
// generic functions.
func mapFunc[T, U any](s catena.Seq[T], f func(T) U) catena.Seq[U] {
	return func(yield func(U) bool) {
		for v := range s.Seq() {
			if !yield(f(v)) {
				return
			}
		}
	}
}

func BenchmarkMapAsGenericMethod(b *testing.B) {
	s := catena.FromSlice(benchData)
	double := func(v int) int { return v * 2 }
	b.ReportAllocs()
	for b.Loop() {
		benchSink = catena.Sum(s.Map(double))
	}
}

func BenchmarkMapAsPackageFunction(b *testing.B) {
	s := catena.FromSlice(benchData)
	double := func(v int) int { return v * 2 }
	b.ReportAllocs()
	for b.Loop() {
		benchSink = catena.Sum(mapFunc(s, double))
	}
}

func BenchmarkFoldBy(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		m := s.FoldBy(
			func(v int) int { return v % 16 },
			func(int) int { return 0 },
			func(acc, v int) int { return acc + v },
		)
		benchSink = m[0]
	}
}

func BenchmarkGroupByThenFold(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		m := s.GroupBy(func(v int) int { return v % 16 })
		out := make(map[int]int, len(m))
		for k, bucket := range m {
			sum := 0
			for _, v := range bucket {
				sum += v
			}
			out[k] = sum
		}
		benchSink = out[0]
	}
}

func BenchmarkTopNBy(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		benchSink = s.TopNBy(10, catena.Self[int])[0]
	}
}

// The natural hand-written spelling of "top ten": clone, sort, slice. It
// is the honest comparison for TopN, and a different number from
// BenchmarkSortedTakeN below — which measures the same idea expressed
// through catena's own sort, and so carries the pipeline's cost too.
func BenchmarkSortedTakeN_Hand(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sorted := slices.Clone(benchData)
		slices.SortFunc(sorted, func(a, c int) int { return c - a })
		benchSink = sorted[:10][0]
	}
}

func BenchmarkSortedTakeN(b *testing.B) {
	s := catena.FromSlice(benchData)
	b.ReportAllocs()
	for b.Loop() {
		benchSink = catena.SortedDesc(s).Take(10).Collect()[0]
	}
}

func BenchmarkListMapPrealloc(b *testing.B) {
	l := catena.List[int](benchData)
	double := func(v int) int { return v * 2 }
	b.ReportAllocs()
	for b.Loop() {
		benchSink = l.Map(double)[0]
	}
}

func BenchmarkSeqMapCollect(b *testing.B) {
	s := catena.FromSlice(benchData)
	double := func(v int) int { return v * 2 }
	b.ReportAllocs()
	for b.Loop() {
		benchSink = s.Map(double).Collect()[0]
	}
}
