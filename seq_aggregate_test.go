package catena_test

// Conformance registrations for seq_aggregate.go, plus the tie and
// stability policies: first-wins for the Max/Min families,
// encounter order preserved through TopNBy's bounded heap.

import (
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

func init() {
	registerTermOps([]termOp{
		{
			name:   "MaxBy",
			covers: []string{"MaxBy"},
			op:     func(s catena.Seq[int]) any { v, ok := s.MaxBy(func(v int) int { return v % 3 }); return pair2{v, ok} },
			model: func(in []int) any {
				out := pair2{0, false}
				best := 0
				for _, v := range in {
					if k := v % 3; out.B == false || k > best {
						out, best = pair2{v, true}, k
					}
				}
				return out
			},
		},
		{
			name:   "MinBy",
			covers: []string{"MinBy"},
			op:     func(s catena.Seq[int]) any { v, ok := s.MinBy(func(v int) int { return v % 3 }); return pair2{v, ok} },
			model: func(in []int) any {
				out := pair2{0, false}
				best := 0
				for _, v := range in {
					if k := v % 3; out.B == false || k < best {
						out, best = pair2{v, true}, k
					}
				}
				return out
			},
		},
		{
			name:   "MaxOf",
			covers: []string{"MaxOf"},
			op:     func(s catena.Seq[int]) any { k, ok := s.MaxOf(func(v int) int { return -v }); return pair2{k, ok} },
			model: func(in []int) any {
				out := pair2{0, false}
				for _, v := range in {
					if out.B == false || -v > out.A.(int) {
						out = pair2{-v, true}
					}
				}
				return out
			},
		},
		{
			name:   "MinOf",
			covers: []string{"MinOf"},
			op:     func(s catena.Seq[int]) any { k, ok := s.MinOf(func(v int) int { return -v }); return pair2{k, ok} },
			model: func(in []int) any {
				out := pair2{0, false}
				for _, v := range in {
					if out.B == false || -v < out.A.(int) {
						out = pair2{-v, true}
					}
				}
				return out
			},
		},
		{
			name:   "MinMaxOf",
			covers: []string{"MinMaxOf"},
			op: func(s catena.Seq[int]) any {
				min, max, ok := s.MinMaxOf(catena.Self[int])
				return [3]any{min, max, ok}
			},
			model: func(in []int) any {
				if len(in) == 0 {
					return [3]any{0, 0, false}
				}
				min, max := in[0], in[0]
				for _, v := range in[1:] {
					if v < min {
						min = v
					}
					if v > max {
						max = v
					}
				}
				return [3]any{min, max, true}
			},
		},
		{
			name:   "MaxWith",
			covers: []string{"MaxWith"},
			op: func(s catena.Seq[int]) any {
				v, ok := s.MaxWith(func(a, b int) int { return a - b })
				return pair2{v, ok}
			},
			model: func(in []int) any {
				out := pair2{0, false}
				for _, v := range in {
					if out.B == false || v > out.A.(int) {
						out = pair2{v, true}
					}
				}
				return out
			},
		},
		{
			name:   "MinWith",
			covers: []string{"MinWith"},
			op: func(s catena.Seq[int]) any {
				v, ok := s.MinWith(func(a, b int) int { return a - b })
				return pair2{v, ok}
			},
			model: func(in []int) any {
				out := pair2{0, false}
				for _, v := range in {
					if out.B == false || v < out.A.(int) {
						out = pair2{v, true}
					}
				}
				return out
			},
		},
		{
			name:   "TopNBy",
			covers: []string{"TopNBy"},
			op:     func(s catena.Seq[int]) any { return s.TopNBy(3, catena.Self[int]) },
			model: func(in []int) any {
				// stable reference: sort desc (stable), take 3
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
			name:   "BottomNBy",
			covers: []string{"BottomNBy"},
			op:     func(s catena.Seq[int]) any { return s.BottomNBy(3, catena.Self[int]) },
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
			name:   "SumOf",
			covers: []string{"SumOf"},
			op:     func(s catena.Seq[int]) any { return s.SumOf(func(v int) int { return v * 2 }) },
			model: func(in []int) any {
				sum := 0
				for _, v := range in {
					sum += v * 2
				}
				return sum
			},
		},
		{
			name:   "ProductOf",
			covers: []string{"ProductOf"},
			op:     func(s catena.Seq[int]) any { return s.ProductOf(func(v int) int { return v + 1 }) },
			model: func(in []int) any {
				prod := 1
				for _, v := range in {
					prod *= v + 1
				}
				return prod
			},
		},
		{
			name:   "AverageOf",
			covers: []string{"AverageOf"},
			op:     func(s catena.Seq[int]) any { avg, ok := s.AverageOf(catena.Self[int]); return pair2{avg, ok} },
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
			name:   "JoinToString",
			covers: []string{"JoinToString"},
			op: func(s catena.Seq[int]) any {
				return s.JoinToString(",", func(v int) string { return string(rune('a' + ((v%26)+26)%26)) })
			},
			model: func(in []int) any {
				out := ""
				for i, v := range in {
					if i > 0 {
						out += ","
					}
					out += string(rune('a' + ((v%26)+26)%26))
				}
				return out
			},
		},
	}...)
}

// ---- ordering, ties, collisions ----

func TestTiePolicies(t *testing.T) {
	type item struct {
		key  int
		name string
	}
	items := catena.Of(item{1, "first"}, item{2, "second-max"}, item{2, "late-max"}, item{1, "late"})
	t.Run("MaxBy_first_wins", func(t *testing.T) {
		v, _ := items.MaxBy(func(i item) int { return i.key })
		if v.name != "second-max" {
			t.Fatalf("got %v", v)
		}
	})
	t.Run("MinBy_first_wins", func(t *testing.T) {
		v, _ := items.MinBy(func(i item) int { return i.key })
		if v.name != "first" {
			t.Fatalf("got %v", v)
		}
	})
	t.Run("MaxWith_MinWith_first_wins", func(t *testing.T) {
		cmp := func(a, b item) int { return a.key - b.key }
		if v, _ := items.MaxWith(cmp); v.name != "second-max" {
			t.Fatalf("MaxWith: %v", v)
		}
		if v, _ := items.MinWith(cmp); v.name != "first" {
			t.Fatalf("MinWith: %v", v)
		}
	})
	t.Run("IndexBy_last_wins", func(t *testing.T) {
		m := items.IndexBy(func(i item) int { return i.key })
		if m[1].name != "late" || m[2].name != "late-max" {
			t.Fatalf("got %v", m)
		}
	})
	t.Run("Distinct_first_wins", func(t *testing.T) {
		got := items.DistinctBy(func(i item) int { return i.key }).Collect()
		if len(got) != 2 || got[0].name != "first" || got[1].name != "second-max" {
			t.Fatalf("got %v", got)
		}
	})
}

func TestTopNStability(t *testing.T) {
	type item struct {
		key  int
		name string
	}
	items := catena.Of(item{5, "a"}, item{9, "x"}, item{5, "b"}, item{9, "y"}, item{5, "c"}, item{1, "z"})
	t.Run("equal_keys_retain_encounter_order", func(t *testing.T) {
		got := items.TopNBy(4, func(i item) int { return i.key })
		names := make([]string, len(got))
		for i, g := range got {
			names[i] = g.name
		}
		// sorted desc by key; ties in encounter order
		if !slices.Equal(names, []string{"x", "y", "a", "b"}) {
			t.Fatalf("got %v", names)
		}
	})
	t.Run("boundary_ties_first_wins", func(t *testing.T) {
		got := items.TopNBy(3, func(i item) int { return i.key })
		names := make([]string, len(got))
		for i, g := range got {
			names[i] = g.name
		}
		// the third slot is contested by a, b, c (all key 5): a, the
		// earliest, must survive
		if !slices.Equal(names, []string{"x", "y", "a"}) {
			t.Fatalf("got %v", names)
		}
	})
	t.Run("bottom", func(t *testing.T) {
		got := items.BottomNBy(3, func(i item) int { return i.key })
		names := make([]string, len(got))
		for i, g := range got {
			names[i] = g.name
		}
		if !slices.Equal(names, []string{"z", "a", "b"}) {
			t.Fatalf("got %v", names)
		}
	})
	t.Run("n_larger_than_input", func(t *testing.T) {
		got := catena.TopN(catena.Of(3, 1, 2), 10)
		if !slices.Equal(got, []int{3, 2, 1}) {
			t.Fatalf("got %v", got)
		}
	})
}

// BottomN is TopN's mirror and routes through the same bounded heap, so the
// properties worth pinning are the ones the desc flag inverts: the sort
// direction, which end survives the cut, and that equal elements at the
// boundary still resolve to the earliest seen.
func TestBottomNStability(t *testing.T) {
	t.Run("ascending_and_bounded", func(t *testing.T) {
		got := catena.BottomN(catena.Of(5, 1, 9, 3, 7), 3)
		if !slices.Equal(got, []int{1, 3, 5}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("n_larger_than_input", func(t *testing.T) {
		got := catena.BottomN(catena.Of(3, 1, 2), 10)
		if !slices.Equal(got, []int{1, 2, 3}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("zero_and_empty", func(t *testing.T) {
		if got := catena.BottomN(catena.Of(3, 1, 2), 0); got != nil {
			t.Fatalf("n=0: got %v, want nil", got)
		}
		if got := catena.BottomN(catena.Empty[int](), 3); got != nil {
			t.Fatalf("empty: got %v, want nil", got)
		}
	})
	t.Run("agrees_with_sort", func(t *testing.T) {
		in := []int{8, 3, 3, 9, 1, 5, 3, 7, 0, 4}
		want := slices.Clone(in)
		slices.Sort(want)
		for n := range len(in) + 2 {
			got := catena.BottomN(catena.FromSlice(in), n)
			exp := want[:min(n, len(want))]
			if len(got) == 0 && len(exp) == 0 {
				continue
			}
			if !slices.Equal(got, exp) {
				t.Fatalf("n=%d: got %v, want %v", n, got, exp)
			}
		}
	})
}

func TestJoinToStringEmpty(t *testing.T) {
	if got := catena.Empty[int]().JoinToString(",", func(int) string { return "x" }); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := catena.Join(nil, ","); got != "" {
		t.Fatalf("Join nil: %q", got)
	}
}
