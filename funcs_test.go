package catena_test

// Conformance registrations for funcs.go — the constraint-bound package
// functions — plus Equal's pull-side hygiene and the NaN convention
// (§4.12) shared by the whole ordered-aggregation surface.

import (
	"math"
	"runtime"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

func init() {
	registerSeqOps([]seqOp{
		{
			name:   "Distinct",
			covers: []string{"Distinct"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.Distinct(s) },
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					if !slices.Contains(out, v) {
						out = append(out, v)
					}
				}
				return out
			},
			alloc: allocUnbounded,
		},
		{
			name:   "Dedupe",
			covers: []string{"Dedupe"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.Dedupe(s) },
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
			name:   "Sorted",
			covers: []string{"Sorted"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.Sorted(s) },
			model: func(in []int) []int {
				out := slices.Clone(in)
				slices.Sort(out)
				return out
			},
			drains: true,
			alloc:  allocUnbounded,
		},
		{
			name:   "SortedDesc",
			covers: []string{"SortedDesc"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.SortedDesc(s) },
			model: func(in []int) []int {
				out := slices.Clone(in)
				slices.Sort(out)
				slices.Reverse(out)
				return out
			},
			drains: true,
			alloc:  allocUnbounded,
		},
		{
			name:   "NonZero",
			covers: []string{"NonZero"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.NonZero(s) },
			model:  func(in []int) []int { return modelFilter(in, func(v int) bool { return v != 0 }) },
		},
		{
			name:   "Union",
			covers: []string{"Union"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.Union(s, catena.Of(1, 100, 1, 101)) },
			model: func(in []int) []int {
				var out []int
				for _, v := range append(slices.Clone(in), 1, 100, 1, 101) {
					if !slices.Contains(out, v) {
						out = append(out, v)
					}
				}
				return out
			},
			alloc: allocUnbounded,
		},
		{
			name:   "Intersect",
			covers: []string{"Intersect"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.Intersect(s, catena.Of(1, 2, 3, 4, 5)) },
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					if v >= 1 && v <= 5 && !slices.Contains(out, v) {
						out = append(out, v)
					}
				}
				return out
			},
			alloc: allocUnbounded,
		},
		{
			name:   "Except",
			covers: []string{"Except"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.Except(s, catena.Of(1, 2, 3, 4, 5)) },
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					if (v < 1 || v > 5) && !slices.Contains(out, v) {
						out = append(out, v)
					}
				}
				return out
			},
			alloc: allocUnbounded,
		},
		{
			name:   "Chain",
			covers: []string{"Chain"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return catena.Chain(s, catena.Of(100), nil) },
			model:  func(in []int) []int { return append(slices.Clone(in), 100) },
		},
		{
			name:   "Flatten",
			covers: []string{"Flatten"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return catena.Flatten(s.Map(func(v int) catena.Seq[int] { return catena.Of(v, -v) }))
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
	}...)
	registerTermOps([]termOp{
		{
			name:   "Sum",
			covers: []string{"Sum"},
			op:     func(s catena.Seq[int]) any { return catena.Sum(s) },
			model: func(in []int) any {
				sum := 0
				for _, v := range in {
					sum += v
				}
				return sum
			},
		},
		{
			name:   "Product",
			covers: []string{"Product"},
			op:     func(s catena.Seq[int]) any { return catena.Product(s) },
			model: func(in []int) any {
				prod := 1
				for _, v := range in {
					prod *= v
				}
				return prod
			},
		},
		{
			name:   "Average",
			covers: []string{"Average"},
			op:     func(s catena.Seq[int]) any { avg, ok := catena.Average(s); return pair2{avg, ok} },
			model: func(in []int) any {
				if len(in) == 0 {
					return pair2{0.0, false}
				}
				sum := 0.0
				for _, v := range in {
					sum += float64(v)
				}
				return pair2{sum / float64(len(in)), true}
			},
		},
		{
			name:   "Max",
			covers: []string{"Max"},
			op:     func(s catena.Seq[int]) any { v, ok := catena.Max(s); return pair2{v, ok} },
			model: func(in []int) any {
				if len(in) == 0 {
					return pair2{0, false}
				}
				return pair2{slices.Max(in), true}
			},
		},
		{
			name:   "Min",
			covers: []string{"Min"},
			op:     func(s catena.Seq[int]) any { v, ok := catena.Min(s); return pair2{v, ok} },
			model: func(in []int) any {
				if len(in) == 0 {
					return pair2{0, false}
				}
				return pair2{slices.Min(in), true}
			},
		},
		{
			name:   "MinMax",
			covers: []string{"MinMax"},
			op: func(s catena.Seq[int]) any {
				min, max, ok := catena.MinMax(s)
				return [3]any{min, max, ok}
			},
			model: func(in []int) any {
				if len(in) == 0 {
					return [3]any{0, 0, false}
				}
				return [3]any{slices.Min(in), slices.Max(in), true}
			},
		},
		{
			name:   "TopN",
			covers: []string{"TopN"},
			op:     func(s catena.Seq[int]) any { return catena.TopN(s, 3) },
			model: func(in []int) any {
				type e struct{ v, i int }
				es := make([]e, len(in))
				for i, v := range in {
					es[i] = e{v, i}
				}
				slices.SortStableFunc(es, func(a, b e) int { return b.v - a.v })
				if len(es) > 3 {
					es = es[:3]
				}
				var out []int
				for _, x := range es {
					out = append(out, x.v)
				}
				return out
			},
		},
		{
			name:   "BottomN",
			covers: []string{"BottomN"},
			op:     func(s catena.Seq[int]) any { return catena.BottomN(s, 3) },
			model: func(in []int) any {
				type e struct{ v, i int }
				es := make([]e, len(in))
				for i, v := range in {
					es[i] = e{v, i}
				}
				slices.SortStableFunc(es, func(a, b e) int { return a.v - b.v })
				if len(es) > 3 {
					es = es[:3]
				}
				var out []int
				for _, x := range es {
					out = append(out, x.v)
				}
				return out
			},
		},
		{
			name:   "Contains",
			covers: []string{"Contains"},
			op:     func(s catena.Seq[int]) any { return catena.Contains(s, 3) },
			model:  func(in []int) any { return slices.Contains(in, 3) },
			infOp:  func(s catena.Seq[int]) any { return catena.Contains(s, 3) },
		},
		{
			name:   "IndexOf",
			covers: []string{"IndexOf"},
			op:     func(s catena.Seq[int]) any { return catena.IndexOf(s, 3) },
			model:  func(in []int) any { return slices.Index(in, 3) },
			infOp:  func(s catena.Seq[int]) any { return catena.IndexOf(s, 3) },
		},
		{
			name:   "ToKeySet",
			covers: []string{"ToKeySet"},
			op:     func(s catena.Seq[int]) any { return catena.ToKeySet(s) },
			model: func(in []int) any {
				out := map[int]struct{}{}
				for _, v := range in {
					out[v] = struct{}{}
				}
				return out
			},
		},
		{
			name:   "Tally",
			covers: []string{"Tally"},
			op:     func(s catena.Seq[int]) any { return catena.Tally(s) },
			model: func(in []int) any {
				out := map[int]int{}
				for _, v := range in {
					out[v]++
				}
				return out
			},
		},
		{
			name:   "AssociateWith",
			covers: []string{"AssociateWith"},
			op:     func(s catena.Seq[int]) any { return catena.AssociateWith(s, func(v int) int { return v * 10 }) },
			model: func(in []int) any {
				out := map[int]int{}
				for _, v := range in {
					out[v] = v * 10
				}
				return out
			},
		},
		{
			name:   "Join",
			covers: []string{"Join"},
			op: func(s catena.Seq[int]) any {
				return catena.Join(s.Map(func(v int) string { return string(rune('a' + ((v%26)+26)%26)) }), "-")
			},
			model: func(in []int) any {
				out := ""
				for i, v := range in {
					if i > 0 {
						out += "-"
					}
					out += string(rune('a' + ((v%26)+26)%26))
				}
				return out
			},
		},
		{
			name:   "CollectMap",
			covers: []string{"CollectMap"},
			op:     func(s catena.Seq[int]) any { return catena.CollectMap(s.WithIndex()) },
			model: func(in []int) any {
				out := map[int]int{}
				for i, v := range in {
					out[i] = v
				}
				return out
			},
		},
		{
			name:   "Unzip",
			covers: []string{"Unzip"},
			op: func(s catena.Seq[int]) any {
				ks, vs := catena.Unzip(s.WithIndex())
				return pair2{ks, vs}
			},
			model: func(in []int) any {
				var ks, vs []int
				for i, v := range in {
					ks = append(ks, i)
					vs = append(vs, v)
				}
				return pair2{ks, vs}
			},
		},
	}...)
}

func TestEqual(t *testing.T) {
	if !catena.Equal(catena.Of(1, 2, 3), catena.Of(1, 2, 3)) {
		t.Fatal("equal reported unequal")
	}
	if catena.Equal(catena.Of(1, 2, 3), catena.Of(1, 2)) {
		t.Fatal("prefix reported equal")
	}
	if catena.Equal(catena.Of(1, 2), catena.Of(1, 2, 3)) {
		t.Fatal("longer b reported equal")
	}
	if catena.Equal(catena.Of(1, 2), catena.Of(1, 9)) {
		t.Fatal("differing element reported equal")
	}
	if !catena.Equal(catena.Empty[int](), nil) {
		t.Fatal("empty vs nil must be equal")
	}
	t.Run("stops_at_first_difference", func(t *testing.T) {
		mustComplete(t, testTimeout, func() {
			if catena.Equal(catena.Of(9), infinite()) {
				t.Fatal("reported equal")
			}
		})
	})
	t.Run("no_goroutine_leak", func(t *testing.T) {
		before := runtime.NumGoroutine()
		catena.Equal(catena.Of(1, 2, 3), infinite())
		waitFor(t, func() bool { return runtime.NumGoroutine() <= before })
	})
}

// ---- §4.12 float semantics ----

func TestFloatNaN(t *testing.T) {
	nan := math.NaN()
	s := catena.Of(1.0, nan, 3.0)
	if v, ok := catena.Max(s); !ok || v != 3.0 {
		t.Fatalf("Max with NaN: %v %v", v, ok)
	}
	if v, ok := catena.Min(s); !ok || !math.IsNaN(v) {
		t.Fatalf("Min with NaN must be NaN: %v %v", v, ok)
	}
	sorted := catena.Sorted(s).Collect()
	if !math.IsNaN(sorted[0]) || sorted[1] != 1.0 || sorted[2] != 3.0 {
		t.Fatalf("NaN must sort first: %v", sorted)
	}
	if k, ok := s.MaxOf(catena.Self[float64]); !ok || k != 3.0 {
		t.Fatalf("MaxOf: %v", k)
	}
	if k, ok := s.MinOf(catena.Self[float64]); !ok || !math.IsNaN(k) {
		t.Fatalf("MinOf: %v", k)
	}
	top := s.TopNBy(2, catena.Self[float64])
	if !slices.Equal(top[:1], []float64{3.0}) || top[1] != 1.0 {
		t.Fatalf("TopNBy with NaN: %v", top)
	}
	if !math.IsNaN(catena.Sum(catena.Of(1.0, nan))) {
		t.Fatal("Sum must propagate NaN")
	}
}
