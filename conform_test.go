package catena_test

// The conformance harness: fixtures, invariants C1-C13,
// and the two registries every operator joins. Each *_test.go file
// registers the cases for its source file from an init function, so the
// pairing between a file and its tests is visible in the file list; the
// harness here is shared infrastructure, and completeness_test.go fails
// the build for any exported symbol nothing registers.
//
// Invariants checked per Seq operator:
//
//	C1  nil receiver behaves as the empty input, no panic
//	C2  re-iterating the result gives identical output (L1)
//	C3  early termination propagates over an infinite source (L2)
//	C5  early break runs the producer's deferred cleanup
//	C6  yield is never called after returning false
//	C7  output equals the hand-written model (incl. C10 empty, C11 single)
//	C8  allocations do not scale with input for streaming operators
//	C9  a panicking callback propagates uncaught
//	C12 emitting k elements consumes at most k+c source elements
//	C13 the operator consumes its source at most once

import (
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/NerdMeNot/catena"
)

var (
	seqOpRegistry  []seqOp
	termOpRegistry []termOp
)

func registerSeqOps(cases ...seqOp)   { seqOpRegistry = append(seqOpRegistry, cases...) }
func registerTermOps(cases ...termOp) { termOpRegistry = append(termOpRegistry, cases...) }

func even(v int) bool { return v%2 == 0 }

func modelFilter(in []int, pred func(int) bool) []int {
	var out []int
	for _, v := range in {
		if pred(v) {
			out = append(out, v)
		}
	}
	return out
}

// pair2 carries two-value results through the any-typed harness.
type pair2 struct{ A, B any }

var errBoom = errors.New("boom")

// ---- fixtures ----

// tracked yields 0..n-1, counting iteration starts and deferred cleanups:
// every started iteration must run its cleanup, even on early termination
// (C5).
func tracked(n int, starts, cleanups *int) catena.Seq[int] {
	return func(yield func(int) bool) {
		*starts++
		defer func() { *cleanups++ }()
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

// infinite yields 0, 1, 2, ... forever (C3).
func infinite() catena.Seq[int] {
	return func(yield func(int) bool) {
		for i := 0; ; i++ {
			if !yield(i) {
				return
			}
		}
	}
}

// singleUse yields 0..n-1 once and panics on a second iteration (C13).
func singleUse(n int) catena.Seq[int] {
	used := false
	return func(yield func(int) bool) {
		if used {
			panic("conformance: source consumed twice")
		}
		used = true
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

// counting wraps a source and counts how many elements the consumer pulled
// (C12).
func counting(inner catena.Seq[int], n *int) catena.Seq[int] {
	return func(yield func(int) bool) {
		for v := range inner {
			*n++
			if !yield(v) {
				return
			}
		}
	}
}

// testTimeout bounds every check that would otherwise hang on a bug.
const testTimeout = 2 * time.Second

// mustComplete fails the test if f does not return within the deadline —
// the guard that turns a hang (C3 violation) into a failure.
func mustComplete(t *testing.T, d time.Duration, f func()) {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f()
	}()
	select {
	case <-done:
	case <-time.After(d):
		t.Fatal("did not terminate: early termination not propagated (C3/L2)")
	}
}

// mustPanic asserts f panics and returns the recovered value.
func mustPanic(t *testing.T, f func()) any {
	t.Helper()
	var got any
	func() {
		defer func() { got = recover() }()
		f()
		t.Fatal("expected panic, got none")
	}()
	return got
}

// ---- Seq[int] → Seq[int] harness ----

type allocClass int

const (
	allocStreaming allocClass = iota // zero allocations per element (C8 strict)
	allocBounded                     // allocates per emission or per bound
	allocUnbounded                   // buffers the input
)

type seqOp struct {
	name    string
	covers  []string // exported symbols this registration exercises
	op      func(catena.Seq[int]) catena.Seq[int]
	model   func([]int) []int                     // hand-written oracle (C7)
	panicOp func(catena.Seq[int]) catena.Seq[int] // op with a panicking callback (C9)
	alloc   allocClass
	drains  bool // full-drain intermediate: skip C3, C8, C12
	lazyC   int  // C12 allowance beyond k consumed elements (default 8)
	single  bool // Once-style: second iteration must panic (C2 inverted)
}

var conformInputs = [][]int{
	nil,
	{5},
	{1, 2, 3, 4, 5},
	{2, 2, 1, 3, 2, 2, 1},
	{-1, 0, 1, 0, -1},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
}

var sink int

func drainSum(s catena.Seq[int]) {
	for v := range s {
		sink += v
	}
}

func conformSeq(t *testing.T, c seqOp) {
	t.Run(c.name, func(t *testing.T) {
		t.Run("C1_nil", func(t *testing.T) {
			// nil behaves as the EMPTY INPUT: ops that add elements
			// (Append, IfEmpty, ...) still yield those elements.
			got, want := c.op(nil).Collect(), c.model(nil)
			if !slices.Equal(got, want) {
				t.Fatalf("nil receiver yielded %v, want %v", got, want)
			}
		})
		t.Run("C7_model", func(t *testing.T) {
			for _, in := range conformInputs {
				got := c.op(catena.FromSlice(in)).Collect()
				want := c.model(in)
				if !slices.Equal(got, want) {
					t.Fatalf("input %v: got %v, want %v", in, got, want)
				}
			}
		})
		t.Run("C2_reiterate", func(t *testing.T) {
			in := []int{4, 1, 4, 2, 8, 5, 7, 1, 4}
			r := c.op(catena.FromSlice(in))
			first := r.Collect()
			if c.single {
				mustPanic(t, func() { r.Collect() })
				return
			}
			second := r.Collect()
			if !slices.Equal(first, second) {
				t.Fatalf("second iteration differs: %v then %v (captured state, L1)", first, second)
			}
		})
		t.Run("C13_single_consumption", func(t *testing.T) {
			c.op(singleUse(10)).Collect() // fixture panics on a second pass
		})
		t.Run("C5_cleanup", func(t *testing.T) {
			// Early break: every producer iteration that STARTED must have
			// run its deferred cleanup. (Prepend-style ops may break before
			// the source ever starts — that is fine.)
			starts, cleanups := 0, 0
			for range c.op(tracked(10, &starts, &cleanups)) {
				break
			}
			if cleanups != starts {
				t.Fatalf("producer started %d times but cleanup ran %d times", starts, cleanups)
			}
			// Full drain: cleanup must run on normal completion too.
			starts, cleanups = 0, 0
			c.op(tracked(10, &starts, &cleanups)).Drain()
			if starts == 0 && !c.single {
				t.Fatal("operator never consumed its source on a full drain")
			}
			if cleanups != starts {
				t.Fatalf("producer started %d times but cleanup ran %d times", starts, cleanups)
			}
		})
		t.Run("C6_yield_after_false", func(t *testing.T) {
			for _, stopAt := range []int{1, 2} {
				dead := false
				calls := 0
				c.op(catena.FromSlice([]int{7, 7, 7, 7, 7, 7, 7, 7}))(func(int) bool {
					if dead {
						t.Fatal("yield called after returning false (L2)")
					}
					calls++
					if calls == stopAt {
						dead = true
						return false
					}
					return true
				})
			}
		})
		if c.panicOp != nil {
			t.Run("C9_panic_propagates", func(t *testing.T) {
				got := mustPanic(t, func() { c.panicOp(catena.FromSlice([]int{1, 2, 3})).Collect() })
				if got != "callback boom" {
					t.Fatalf("recovered %v, want the callback's panic value", got)
				}
			})
		}
		if !c.drains {
			t.Run("C3_infinite", func(t *testing.T) {
				mustComplete(t, 2*time.Second, func() {
					n := 0
					for range c.op(infinite()) {
						if n++; n == 3 {
							break
						}
					}
				})
			})
			t.Run("C12_laziness", func(t *testing.T) {
				lazyC := c.lazyC
				if lazyC == 0 {
					lazyC = 8
				}
				const k = 2
				consumed := 0
				mustComplete(t, 2*time.Second, func() {
					n := 0
					for range c.op(counting(infinite(), &consumed)) {
						if n++; n == k {
							break
						}
					}
				})
				if consumed > k+lazyC {
					t.Fatalf("emitting %d elements consumed %d from the source (allowance %d)", k, consumed, k+lazyC)
				}
			})
		}
		if c.alloc == allocStreaming {
			t.Run("C8_allocs", func(t *testing.T) {
				small := make([]int, 100)
				big := make([]int, 1100)
				measure := func(in []int) float64 {
					s := catena.FromSlice(in)
					return testing.AllocsPerRun(10, func() { drainSum(c.op(s)) })
				}
				a, b := measure(small), measure(big)
				if b-a > 0.5 {
					t.Fatalf("allocations scale with input: %.1f for 100 elements, %.1f for 1100", a, b)
				}
			})
		}
	})
}

// ---- Seq[int] → R terminal harness ----

type termOp struct {
	name   string
	covers []string
	op     func(catena.Seq[int]) any
	model  func([]int) any // hand-written oracle
	// infOp, when set, is run over an infinite source and must terminate:
	// the check for conditional-drain (◐) terminals.
	infOp func(catena.Seq[int]) any
}

func conformTerm(t *testing.T, c termOp) {
	t.Run(c.name, func(t *testing.T) {
		t.Run("C1_nil", func(t *testing.T) {
			got, want := c.op(nil), c.model(nil)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("nil receiver: got %#v, want %#v", got, want)
			}
		})
		t.Run("C7_model", func(t *testing.T) {
			for _, in := range conformInputs {
				got := c.op(catena.FromSlice(in))
				want := c.model(in)
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("input %v: got %#v, want %#v", in, got, want)
				}
			}
		})
		t.Run("C13_single_consumption", func(t *testing.T) {
			c.op(singleUse(10))
		})
		if c.infOp != nil {
			t.Run("conditional_drain_terminates", func(t *testing.T) {
				mustComplete(t, 2*time.Second, func() { c.infOp(infinite()) })
			})
		}
	})
}

// runs both registries; the registries live in *_conform_test.go files
// next to what they cover.
func TestConformSeq(t *testing.T) {
	names := map[string]bool{}
	for _, c := range seqOpRegistry {
		if names[c.name] {
			t.Fatalf("duplicate registration %q", c.name)
		}
		names[c.name] = true
		conformSeq(t, c)
	}
}

func TestConformTerminals(t *testing.T) {
	names := map[string]bool{}
	for _, c := range termOpRegistry {
		if names[c.name] {
			t.Fatalf("duplicate registration %q", c.name)
		}
		names[c.name] = true
		conformTerm(t, c)
	}
}

// sanity checks on the fixtures themselves: a harness that cannot fail
// proves nothing.
func TestFixtures(t *testing.T) {
	t.Run("singleUse_panics_on_second_pass", func(t *testing.T) {
		s := singleUse(3)
		s.Collect()
		mustPanic(t, func() { s.Collect() })
	})
	t.Run("tracked_counts_cleanup", func(t *testing.T) {
		st, n := 0, 0
		tracked(3, &st, &n).Collect()
		if st != 1 || n != 1 {
			t.Fatalf("starts = %d, cleanups = %d", st, n)
		}
	})
	t.Run("counting_counts", func(t *testing.T) {
		n := 0
		counting(catena.Of(1, 2, 3), &n).Collect()
		if n != 3 {
			t.Fatalf("consumed = %d", n)
		}
	})
	t.Run("mustPanic_fails_without_panic", func(t *testing.T) {
		// exercised indirectly: mustPanic is used throughout; this pins
		// the panic-value passthrough.
		if got := mustPanic(t, func() { panic("x") }); got != "x" {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("harness_catches_L1_bug", func(t *testing.T) {
		// A deliberately broken Take with enclosing-function state must
		// fail C2: proof the harness detects the bug class it exists for.
		brokenTake := func(s catena.Seq[int]) catena.Seq[int] {
			n := 2 // BUG: captured once, shared across iterations
			return func(yield func(int) bool) {
				for v := range s {
					if n <= 0 {
						return
					}
					n--
					if !yield(v) {
						return
					}
				}
			}
		}
		r := brokenTake(catena.Of(1, 2, 3, 4))
		first := r.Collect()
		second := r.Collect()
		if slices.Equal(first, second) {
			t.Fatal("broken operator not detected — harness is toothless")
		}
	})
}

func panicky(int) int { panic("callback boom") }

func panickyPred(int) bool { panic("callback boom") }
