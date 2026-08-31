package catena_test

// Conformance registrations for seq_map.go — the type-changing
// intermediates — plus Zip's consumption contract, which the harness
// cannot see from the outside.

import (
	"runtime"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

func init() {
	registerSeqOps([]seqOp{
		{
			name:   "Map",
			covers: []string{"Map"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Map(func(v int) int { return v * 2 }) },
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					out = append(out, v*2)
				}
				return out
			},
			panicOp: func(s catena.Seq[int]) catena.Seq[int] { return s.Map(panicky) },
		},
		{
			name:   "MapIndexed",
			covers: []string{"MapIndexed"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.MapIndexed(func(i, v int) int { return i*1000 + v })
			},
			model: func(in []int) []int {
				var out []int
				for i, v := range in {
					out = append(out, i*1000+v)
				}
				return out
			},
		},
		{
			name:   "FilterMap",
			covers: []string{"FilterMap"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.FilterMap(func(v int) (int, bool) { return v * 10, even(v) })
			},
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					if even(v) {
						out = append(out, v*10)
					}
				}
				return out
			},
		},
		{
			name:   "FlatMap",
			covers: []string{"FlatMap"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.FlatMap(func(v int) catena.Seq[int] { return catena.Of(v, -v) })
			},
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					out = append(out, v, -v)
				}
				return out
			},
			alloc: allocBounded, // the callback's Of(...) allocates per element
		},
		{
			name:   "FlatMapSlice",
			covers: []string{"FlatMapSlice"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.FlatMapSlice(func(v int) []int { return []int{v, -v} })
			},
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					out = append(out, v, -v)
				}
				return out
			},
			alloc: allocBounded, // the callback allocates its slice per element
		},
		{
			name:   "Scan",
			covers: []string{"Scan"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.Scan(0, func(a, v int) int { return a + v })
			},
			model: func(in []int) []int {
				var out []int
				acc := 0
				for _, v := range in {
					acc += v
					out = append(out, acc)
				}
				return out
			},
		},
		{
			name:   "DistinctBy",
			covers: []string{"DistinctBy"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.DistinctBy(func(v int) int { return v % 5 }) },
			model: func(in []int) []int {
				seen := map[int]bool{}
				var out []int
				for _, v := range in {
					if !seen[v%5] {
						seen[v%5] = true
						out = append(out, v)
					}
				}
				return out
			},
			alloc: allocUnbounded,
		},
		{
			name:   "DedupeBy",
			covers: []string{"DedupeBy"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.DedupeBy(catena.Self[int]) },
			model: func(in []int) []int {
				var out []int
				for i, v := range in {
					if i == 0 || v != in[i-1] {
						out = append(out, v)
					}
				}
				return out
			},
		},
		{
			name:   "SortedBy",
			covers: []string{"SortedBy"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.SortedBy(func(v int) int { return v % 3 }) },
			model: func(in []int) []int {
				out := slices.Clone(in)
				slices.SortStableFunc(out, func(a, b int) int { return a%3 - b%3 })
				return out
			},
			drains: true,
			alloc:  allocUnbounded,
		},
		{
			name:   "SortedByDesc",
			covers: []string{"SortedByDesc"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.SortedByDesc(func(v int) int { return v % 3 }) },
			model: func(in []int) []int {
				out := slices.Clone(in)
				slices.SortStableFunc(out, func(a, b int) int { return b%3 - a%3 })
				return out
			},
			drains: true,
			alloc:  allocUnbounded,
		},
		{
			name:   "Zip_MapTo",
			covers: []string{"Zip", "MapTo"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.Zip(catena.Range(0, 1000, 1)).MapTo(func(v, i int) int { return v + i*1000 })
			},
			model: func(in []int) []int {
				var out []int
				for i, v := range in {
					out = append(out, v+i*1000)
				}
				return out
			},
		},
		{
			name:   "JoinBy",
			covers: []string{"JoinBy"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.JoinBy(catena.Of(0, 1, 1, 2),
					func(v int) int { return ((v % 3) + 3) % 3 },
					catena.Self[int],
					func(v, u int) int { return v*10 + u })
			},
			model: func(in []int) []int {
				right := map[int][]int{0: {0}, 1: {1, 1}, 2: {2}}
				var out []int
				for _, v := range in {
					for _, u := range right[((v%3)+3)%3] {
						out = append(out, v*10+u)
					}
				}
				return out
			},
			alloc: allocBounded,
		},
		{
			name:   "MapErr_Ignore",
			covers: []string{"MapErr", "Ignore"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.MapErr(func(v int) (int, error) { return v + 1, nil }).Ignore()
			},
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					out = append(out, v+1)
				}
				return out
			},
		},
		{
			name:   "FilterErr_Ignore",
			covers: []string{"FilterErr"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.FilterErr(func(v int) (bool, error) { return even(v), nil }).Ignore()
			},
			model: func(in []int) []int { return modelFilter(in, even) },
		},
		{
			name:   "WithIndex_MapTo",
			covers: []string{"WithIndex"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.WithIndex().MapTo(func(i, v int) int { return i*1000 + v })
			},
			model: func(in []int) []int {
				var out []int
				for i, v := range in {
					out = append(out, i*1000+v)
				}
				return out
			},
		},
		{
			name:   "ZipWithNext_MapTo",
			covers: []string{"ZipWithNext"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.ZipWithNext().MapTo(func(a, b int) int { return a*1000 + b })
			},
			model: func(in []int) []int {
				var out []int
				for i := 1; i < len(in); i++ {
					out = append(out, in[i-1]*1000+in[i])
				}
				return out
			},
		},
	}...)
}

func TestZipConsumption(t *testing.T) {
	t.Run("shorter_other", func(t *testing.T) {
		consumed := 0
		s := counting(infinite(), &consumed)
		got := s.Zip(catena.Of("a", "b")).MapTo(func(v int, u string) string { return u }).Collect()
		if !slices.Equal(got, []string{"a", "b"}) {
			t.Fatalf("got %v", got)
		}
		// receiver over-consumes by exactly one when other is shorter
		if consumed != 3 {
			t.Fatalf("receiver consumed %d, want pairs+1 = 3", consumed)
		}
	})
	t.Run("shorter_receiver", func(t *testing.T) {
		pulled := 0
		other := counting(infinite(), &pulled)
		catena.Of(1, 2).Zip(other).MapTo(func(a, b int) int { return a }).Drain()
		if pulled != 2 {
			t.Fatalf("other pulled %d, want exactly the pair count 2", pulled)
		}
	})
	t.Run("no_goroutine_leak_on_early_break", func(t *testing.T) {
		before := runtime.NumGoroutine()
		z := infinite().Zip(infinite()).MapTo(func(a, b int) int { return a + b })
		z.Take(3).Drain()
		waitFor(t, func() bool { return runtime.NumGoroutine() <= before })
	})
}

// Branch closure: error-element emission rejected by the consumer.
func TestSeqMapEarlyBreaks(t *testing.T) {
	t.Run("SeqFilterErr_break_on_error_element", func(t *testing.T) {
		bad := catena.Of(1, 2, 3).FilterErr(func(v int) (bool, error) { return false, e1 })
		if got := collectTry(bad.Take(1)); !slices.Equal(got, []tv{{0, e1}}) {
			t.Fatalf("got %v", got)
		}
	})
}
