package catena_test

// Conformance registrations for seq_terminal.go, plus the resource
// contracts of the escape hatches: Pull's caller-must-stop rule, ToChan's
// goroutine lifecycle, and the terminals whose consumption behavior is
// itself the contract (IsEmpty, Single, ElementAt).

import (
	"context"
	"runtime"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NerdMeNot/catena"
)

func init() {
	registerTermOps([]termOp{
		{
			name:   "Collect",
			covers: []string{"Collect"},
			op:     func(s catena.Seq[int]) any { return s.Collect() },
			model:  func(in []int) any { return slices.Clone(in) },
		},
		{
			name:   "ToList",
			covers: []string{"ToList"},
			op:     func(s catena.Seq[int]) any { return s.ToList() },
			model:  func(in []int) any { return catena.List[int](slices.Clone(in)) },
		},
		{
			name:   "ForEach",
			covers: []string{"ForEach"},
			op: func(s catena.Seq[int]) any {
				sum := 0
				s.ForEach(func(v int) { sum += v })
				return sum
			},
			model: func(in []int) any {
				sum := 0
				for _, v := range in {
					sum += v
				}
				return sum
			},
		},
		{
			name:   "ForEachIndexed",
			covers: []string{"ForEachIndexed"},
			op: func(s catena.Seq[int]) any {
				sum := 0
				s.ForEachIndexed(func(i, v int) { sum += i * v })
				return sum
			},
			model: func(in []int) any {
				sum := 0
				for i, v := range in {
					sum += i * v
				}
				return sum
			},
		},
		{
			name:   "ForEachErr",
			covers: []string{"ForEachErr"},
			op: func(s catena.Seq[int]) any {
				sum := 0
				err := s.ForEachErr(func(v int) error {
					if v == 3 {
						return errBoom
					}
					sum += v
					return nil
				})
				return pair2{sum, err}
			},
			model: func(in []int) any {
				sum := 0
				for _, v := range in {
					if v == 3 {
						return pair2{sum, error(errBoom)}
					}
					sum += v
				}
				return pair2{sum, error(nil)}
			},
			infOp: func(s catena.Seq[int]) any {
				return s.ForEachErr(func(v int) error {
					if v == 3 {
						return errBoom
					}
					return nil
				})
			},
		},
		{
			name:   "Drain",
			covers: []string{"Drain"},
			op:     func(s catena.Seq[int]) any { s.Drain(); return nil },
			model:  func([]int) any { return nil },
		},
		{
			name:   "First",
			covers: []string{"First"},
			op:     func(s catena.Seq[int]) any { v, ok := s.First(); return pair2{v, ok} },
			model: func(in []int) any {
				if len(in) == 0 {
					return pair2{0, false}
				}
				return pair2{in[0], true}
			},
			infOp: func(s catena.Seq[int]) any { v, _ := s.First(); return v },
		},
		{
			name:   "Last",
			covers: []string{"Last"},
			op:     func(s catena.Seq[int]) any { v, ok := s.Last(); return pair2{v, ok} },
			model: func(in []int) any {
				if len(in) == 0 {
					return pair2{0, false}
				}
				return pair2{in[len(in)-1], true}
			},
		},
		{
			name:   "Single",
			covers: []string{"Single"},
			op:     func(s catena.Seq[int]) any { v, ok := s.Single(); return pair2{v, ok} },
			model: func(in []int) any {
				if len(in) == 1 {
					return pair2{in[0], true}
				}
				return pair2{0, false}
			},
			infOp: func(s catena.Seq[int]) any { _, ok := s.Single(); return ok },
		},
		{
			name:   "ElementAt",
			covers: []string{"ElementAt"},
			op:     func(s catena.Seq[int]) any { v, ok := s.ElementAt(3); return pair2{v, ok} },
			model: func(in []int) any {
				if len(in) <= 3 {
					return pair2{0, false}
				}
				return pair2{in[3], true}
			},
			infOp: func(s catena.Seq[int]) any { v, _ := s.ElementAt(3); return v },
		},
		{
			name:   "Find",
			covers: []string{"Find"},
			op:     func(s catena.Seq[int]) any { v, ok := s.Find(func(v int) bool { return v > 2 }); return pair2{v, ok} },
			model: func(in []int) any {
				for _, v := range in {
					if v > 2 {
						return pair2{v, true}
					}
				}
				return pair2{0, false}
			},
			infOp: func(s catena.Seq[int]) any { v, _ := s.Find(func(v int) bool { return v > 2 }); return v },
		},
		{
			name:   "FindLast",
			covers: []string{"FindLast"},
			op: func(s catena.Seq[int]) any {
				v, ok := s.FindLast(func(v int) bool { return v > 2 })
				return pair2{v, ok}
			},
			model: func(in []int) any {
				out := pair2{0, false}
				for _, v := range in {
					if v > 2 {
						out = pair2{v, true}
					}
				}
				return out
			},
		},
		{
			name:   "FindIndex",
			covers: []string{"FindIndex"},
			op:     func(s catena.Seq[int]) any { return s.FindIndex(func(v int) bool { return v > 2 }) },
			model: func(in []int) any {
				for i, v := range in {
					if v > 2 {
						return i
					}
				}
				return -1
			},
			infOp: func(s catena.Seq[int]) any { return s.FindIndex(func(v int) bool { return v > 2 }) },
		},
		{
			name:   "FindMap",
			covers: []string{"FindMap"},
			op: func(s catena.Seq[int]) any {
				v, ok := s.FindMap(func(v int) (int, bool) { return v * 10, v > 2 })
				return pair2{v, ok}
			},
			model: func(in []int) any {
				for _, v := range in {
					if v > 2 {
						return pair2{v * 10, true}
					}
				}
				return pair2{0, false}
			},
			infOp: func(s catena.Seq[int]) any {
				v, _ := s.FindMap(func(v int) (int, bool) { return v, v > 2 })
				return v
			},
		},
		{
			name:   "Any",
			covers: []string{"Any"},
			op:     func(s catena.Seq[int]) any { return s.Any(func(v int) bool { return v > 3 }) },
			model: func(in []int) any {
				for _, v := range in {
					if v > 3 {
						return true
					}
				}
				return false
			},
			infOp: func(s catena.Seq[int]) any { return s.Any(func(v int) bool { return v > 3 }) },
		},
		{
			name:   "All",
			covers: []string{"All"},
			op:     func(s catena.Seq[int]) any { return s.All(func(v int) bool { return v < 3 }) },
			model: func(in []int) any {
				for _, v := range in {
					if v >= 3 {
						return false
					}
				}
				return true
			},
			infOp: func(s catena.Seq[int]) any { return s.All(func(v int) bool { return v < 3 }) },
		},
		{
			name:   "None",
			covers: []string{"None"},
			op:     func(s catena.Seq[int]) any { return s.None(func(v int) bool { return v > 3 }) },
			model: func(in []int) any {
				for _, v := range in {
					if v > 3 {
						return false
					}
				}
				return true
			},
			infOp: func(s catena.Seq[int]) any { return s.None(func(v int) bool { return v > 3 }) },
		},
		{
			name:   "Count",
			covers: []string{"Count"},
			op:     func(s catena.Seq[int]) any { return s.Count() },
			model:  func(in []int) any { return len(in) },
		},
		{
			name:   "CountWhere",
			covers: []string{"CountWhere"},
			op:     func(s catena.Seq[int]) any { return s.CountWhere(even) },
			model: func(in []int) any {
				n := 0
				for _, v := range in {
					if even(v) {
						n++
					}
				}
				return n
			},
		},
		{
			name:   "IsEmpty",
			covers: []string{"IsEmpty"},
			op:     func(s catena.Seq[int]) any { return s.IsEmpty() },
			model:  func(in []int) any { return len(in) == 0 },
			infOp:  func(s catena.Seq[int]) any { return s.IsEmpty() },
		},
	}...)
}

// ---- Pull / ToChan / goroutine hygiene (C4) ----

func TestPull(t *testing.T) {
	next, stop := catena.Of(1, 2).Pull()
	v, ok := next()
	if v != 1 || !ok {
		t.Fatalf("got %v %v", v, ok)
	}
	stop()
	if _, ok := next(); ok {
		t.Fatal("next after stop yielded")
	}
	t.Run("nil", func(t *testing.T) {
		next, stop := catena.Seq[int](nil).Pull()
		defer stop()
		if _, ok := next(); ok {
			t.Fatal("Pull on nil yielded")
		}
	})
	t.Run("stop_runs_producer_cleanup", func(t *testing.T) {
		starts, cleanups := 0, 0
		next, stop := tracked(10, &starts, &cleanups).Pull()
		next()
		stop()
		if cleanups != 1 {
			t.Fatalf("cleanups = %d after stop", cleanups)
		}
	})
}

func TestToChan(t *testing.T) {
	t.Run("drains", func(t *testing.T) {
		var got []int
		for v := range catena.Of(1, 2, 3).ToChan(context.Background()) {
			got = append(got, v)
		}
		if !slices.Equal(got, []int{1, 2, 3}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("nil_is_closed", func(t *testing.T) {
		mustComplete(t, testTimeout, func() {
			for range catena.Seq[int](nil).ToChan(context.Background()) {
				t.Fatal("nil ToChan yielded")
			}
		})
	})
	t.Run("cancel_stops_goroutine_and_runs_cleanup", func(t *testing.T) {
		before := runtime.NumGoroutine()
		ctx, cancel := context.WithCancel(context.Background())
		var cleanups atomic.Int32
		producer := catena.From(func(yield func(int) bool) {
			defer cleanups.Add(1)
			for i := range 1000 {
				if !yield(i) {
					return
				}
			}
		})
		ch := producer.ToChan(ctx)
		<-ch
		cancel()
		waitFor(t, func() bool { return cleanups.Load() == 1 })
		waitFor(t, func() bool { return runtime.NumGoroutine() <= before })
	})
}

// waitFor polls cond until true or the timeout elapses.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(testTimeout)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition never became true")
		}
		time.Sleep(time.Millisecond)
	}
}

// ---- misc contracts ----

func TestIsEmptyConsumesOneElement(t *testing.T) {
	consumed := 0
	s := counting(catena.Of(1, 2, 3), &consumed)
	if s.IsEmpty() {
		t.Fatal("not empty")
	}
	if consumed != 1 {
		t.Fatalf("IsEmpty consumed %d elements, documented as exactly 1", consumed)
	}
}

func TestSingleStopsAtSecondElement(t *testing.T) {
	consumed := 0
	catena.Seq[int](counting(infinite(), &consumed)).Single()
	if consumed != 2 {
		t.Fatalf("Single consumed %d, want 2", consumed)
	}
}

func TestElementAtNegative(t *testing.T) {
	consumed := 0
	v, ok := counting(catena.Of(1, 2, 3), &consumed).ElementAt(-1)
	if v != 0 || ok {
		t.Fatalf("got %v %v", v, ok)
	}
	if consumed != 0 {
		t.Fatalf("negative index consumed %d elements", consumed)
	}
}
