package catena_test

// Conformance registrations for seq.go — the type-preserving intermediates
// and lifecycle guards — plus the operator-specific contracts the harness
// cannot express generically.

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/NerdMeNot/catena"
)

func init() {
	registerSeqOps([]seqOp{
		{
			name:    "Filter",
			covers:  []string{"Filter"},
			op:      func(s catena.Seq[int]) catena.Seq[int] { return s.Filter(even) },
			model:   func(in []int) []int { return modelFilter(in, even) },
			panicOp: func(s catena.Seq[int]) catena.Seq[int] { return s.Filter(panickyPred) },
		},
		{
			name:   "FilterNot",
			covers: []string{"FilterNot"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.FilterNot(even) },
			model:  func(in []int) []int { return modelFilter(in, func(v int) bool { return !even(v) }) },
		},
		{
			name:   "FilterIndexed",
			covers: []string{"FilterIndexed"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.FilterIndexed(func(i, v int) bool { return i%2 == 0 })
			},
			model: func(in []int) []int {
				var out []int
				for i, v := range in {
					if i%2 == 0 {
						out = append(out, v)
					}
				}
				return out
			},
		},
		{
			name:   "Take",
			covers: []string{"Take"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Take(3) },
			model: func(in []int) []int {
				if len(in) > 3 {
					in = in[:3]
				}
				return slices.Clone(in)
			},
			lazyC: 1, // Take must consume exactly what it emits
		},
		{
			name:   "TakeWhile",
			covers: []string{"TakeWhile"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.TakeWhile(func(v int) bool { return v < 8 }) },
			model: func(in []int) []int {
				var out []int
				for _, v := range in {
					if v >= 8 {
						break
					}
					out = append(out, v)
				}
				return out
			},
			panicOp: func(s catena.Seq[int]) catena.Seq[int] { return s.TakeWhile(panickyPred) },
		},
		{
			name:   "TakeLast",
			covers: []string{"TakeLast"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.TakeLast(3) },
			model: func(in []int) []int {
				if len(in) > 3 {
					in = in[len(in)-3:]
				}
				return slices.Clone(in)
			},
			drains: true,
			alloc:  allocBounded,
		},
		{
			name:   "Drop",
			covers: []string{"Drop"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Drop(2) },
			model: func(in []int) []int {
				if len(in) <= 2 {
					return nil
				}
				return slices.Clone(in[2:])
			},
		},
		{
			name:   "DropWhile",
			covers: []string{"DropWhile"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.DropWhile(func(v int) bool { return v < 5 }) },
			model: func(in []int) []int {
				var out []int
				dropping := true
				for _, v := range in {
					if dropping && v < 5 {
						continue
					}
					dropping = false
					out = append(out, v)
				}
				return out
			},
		},
		{
			name:   "DropLast",
			covers: []string{"DropLast"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.DropLast(3) },
			model: func(in []int) []int {
				if len(in) <= 3 {
					return nil
				}
				return slices.Clone(in[:len(in)-3])
			},
			alloc: allocBounded,
		},
		{
			name:   "Step",
			covers: []string{"Step"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Step(3) },
			model: func(in []int) []int {
				var out []int
				for i, v := range in {
					if i%3 == 0 {
						out = append(out, v)
					}
				}
				return out
			},
		},
		{
			name:   "OnEach",
			covers: []string{"OnEach"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.OnEach(func(int) {}) },
			model:  slices.Clone[[]int],
			panicOp: func(s catena.Seq[int]) catena.Seq[int] {
				return s.OnEach(func(int) { panic("callback boom") })
			},
		},
		{
			name:   "Concat",
			covers: []string{"Concat"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Concat(catena.Of(100, 101), nil) },
			model:  func(in []int) []int { return append(slices.Clone(in), 100, 101) },
		},
		{
			name:   "Append",
			covers: []string{"Append"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Append(100, 101) },
			model:  func(in []int) []int { return append(slices.Clone(in), 100, 101) },
		},
		{
			name:   "Prepend",
			covers: []string{"Prepend"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Prepend(100, 101) },
			model:  func(in []int) []int { return append([]int{100, 101}, in...) },
		},
		{
			name:   "Intersperse",
			covers: []string{"Intersperse"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Intersperse(-9) },
			model: func(in []int) []int {
				var out []int
				for i, v := range in {
					if i > 0 {
						out = append(out, -9)
					}
					out = append(out, v)
				}
				return out
			},
		},
		{
			name:   "IfEmpty",
			covers: []string{"IfEmpty"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.IfEmpty(42, 43) },
			model: func(in []int) []int {
				if len(in) == 0 {
					return []int{42, 43}
				}
				return slices.Clone(in)
			},
		},
		{
			name:   "SortedWith",
			covers: []string{"SortedWith"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.SortedWith(func(a, b int) int { return b - a })
			},
			model: func(in []int) []int {
				out := slices.Clone(in)
				slices.SortStableFunc(out, func(a, b int) int { return b - a })
				return out
			},
			drains: true,
			alloc:  allocUnbounded,
		},
		{
			name:   "DistinctWith",
			covers: []string{"DistinctWith"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.DistinctWith(func(a, b int) bool { return a == b })
			},
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
			name:   "Reversed",
			covers: []string{"Reversed"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Reversed() },
			model: func(in []int) []int {
				out := slices.Clone(in)
				slices.Reverse(out)
				return out
			},
			drains: true,
			alloc:  allocUnbounded,
		},
		{
			name:   "Once",
			covers: []string{"Once"},
			op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Once() },
			model:  slices.Clone[[]int],
			single: true,
		},
		{
			name:   "UntilDone_Ignore",
			covers: []string{"UntilDone"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return s.UntilDone(context.Background()).Ignore()
			},
			model: slices.Clone[[]int],
		},
	}...)
}

func TestOncePanicMessage(t *testing.T) {
	s := catena.Of(1).Once()
	s.Drain()
	got := mustPanic(t, func() { s.Drain() })
	msg, ok := got.(string)
	if !ok || !strings.HasPrefix(msg, "catena: Once: ") {
		t.Fatalf("got %v", got)
	}
}

// Branch closure: consumer breaks inside tails and buffered emissions.
func TestSeqEarlyBreaks(t *testing.T) {
	t.Run("DropLast_zero_break", func(t *testing.T) {
		if got := catena.Of(1, 2, 3).DropLast(0).Take(1).Collect(); !slices.Equal(got, []int{1}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("DropLast_break_mid_emission", func(t *testing.T) {
		if got := catena.Range(0, 10, 1).DropLast(2).Take(3).Collect(); !slices.Equal(got, []int{0, 1, 2}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("TakeLast_break_mid_emission", func(t *testing.T) {
		if got := catena.Range(0, 10, 1).TakeLast(5).Take(2).Collect(); !slices.Equal(got, []int{5, 6}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Append_break_in_appended_tail", func(t *testing.T) {
		if got := catena.Of(1).Append(2, 3).Take(2).Collect(); !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Concat_break_in_tail", func(t *testing.T) {
		if got := catena.Of(1).Concat(catena.Of(2, 3)).Take(2).Collect(); !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Prepend_break_in_source", func(t *testing.T) {
		if got := catena.Of(2, 3).Prepend(1).Take(2).Collect(); !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("IfEmpty_break_in_defaults", func(t *testing.T) {
		if got := catena.Empty[int]().IfEmpty(1, 2, 3).Take(2).Collect(); !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Intersperse_break_on_separator", func(t *testing.T) {
		if got := catena.Of(1, 2).Intersperse(0).Take(2).Collect(); !slices.Equal(got, []int{1, 0}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("SeqUntilDone_break_by_consumer", func(t *testing.T) {
		got := collectTry(catena.Of(1, 2, 3).UntilDone(context.Background()).Take(1))
		if !slices.Equal(got, []tv{{1, nil}}) {
			t.Fatalf("got %v", got)
		}
	})
}
