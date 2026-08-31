package catena_test

// C14: the Try invariants (§4.9 R1-R5), verified per operator with
// instrumented callbacks.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

type tv struct {
	v   int
	err error
}

var (
	e1 = errors.New("e1")
	e2 = errors.New("e2")
)

func trySrc(items ...tv) catena.Try[int] {
	return func(yield func(int, error) bool) {
		for _, it := range items {
			if !yield(it.v, it.err) {
				return
			}
		}
	}
}

func collectTry(t catena.Try[int]) []tv {
	var out []tv
	for v, err := range t.Seq2() {
		out = append(out, tv{v, err})
	}
	return out
}

// mixed is the standard fixture: success 1, error e1, success 3, error e2,
// success 5.
func mixed() catena.Try[int] {
	return trySrc(tv{1, nil}, tv{0, e1}, tv{3, nil}, tv{0, e2}, tv{5, nil})
}

func TestTryR1PassThrough(t *testing.T) {
	// R1: callbacks never see errored elements; errored elements flow
	// through untouched.
	t.Run("Filter", func(t *testing.T) {
		var saw []int
		got := collectTry(mixed().Filter(func(v int) bool { saw = append(saw, v); return v != 3 }))
		if !slices.Equal(saw, []int{1, 3, 5}) {
			t.Fatalf("predicate saw %v", saw)
		}
		want := []tv{{1, nil}, {0, e1}, {0, e2}, {5, nil}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Map", func(t *testing.T) {
		var saw []int
		got := collectTry(mixed().Map(func(v int) int { saw = append(saw, v); return v * 10 }))
		if !slices.Equal(saw, []int{1, 3, 5}) {
			t.Fatalf("f saw %v", saw)
		}
		want := []tv{{10, nil}, {0, e1}, {30, nil}, {0, e2}, {50, nil}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("MapErr", func(t *testing.T) {
		var saw []int
		got := collectTry(mixed().MapErr(func(v int) (int, error) { saw = append(saw, v); return v * 10, nil }))
		if !slices.Equal(saw, []int{1, 3, 5}) {
			t.Fatalf("f saw %v", saw)
		}
		want := []tv{{10, nil}, {0, e1}, {30, nil}, {0, e2}, {50, nil}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("FlatMap", func(t *testing.T) {
		var saw []int
		got := collectTry(mixed().FlatMap(func(v int) catena.Try[int] {
			saw = append(saw, v)
			return trySrc(tv{v, nil}, tv{-v, nil})
		}))
		if !slices.Equal(saw, []int{1, 3, 5}) {
			t.Fatalf("f saw %v", saw)
		}
		want := []tv{{1, nil}, {-1, nil}, {0, e1}, {3, nil}, {-3, nil}, {0, e2}, {5, nil}, {-5, nil}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("OnEach", func(t *testing.T) {
		var saw []int
		mixed().OnEach(func(v int) { saw = append(saw, v) }).Err()
		if !slices.Equal(saw, []int{1}) { // Err stops at e1 (R5)
			t.Fatalf("f saw %v", saw)
		}
	})
	t.Run("FilterErr", func(t *testing.T) {
		var saw []int
		got := collectTry(mixed().FilterErr(func(v int) (bool, error) { saw = append(saw, v); return v != 3, nil }))
		if !slices.Equal(saw, []int{1, 3, 5}) {
			t.Fatalf("pred saw %v", saw)
		}
		want := []tv{{1, nil}, {0, e1}, {0, e2}, {5, nil}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestTryR2PositionalCounting(t *testing.T) {
	// R2: errored elements count for Take, Drop, and consumption bounds.
	if got := collectTry(mixed().Take(2)); !slices.Equal(got, []tv{{1, nil}, {0, e1}}) {
		t.Fatalf("Take(2) = %v", got)
	}
	if got := collectTry(mixed().Drop(1)); !slices.Equal(got, []tv{{0, e1}, {3, nil}, {0, e2}, {5, nil}}) {
		t.Fatalf("Drop(1) = %v", got)
	}
	// The "n successes" spelling.
	if got := mixed().Ignore().Take(2).Collect(); !slices.Equal(got, []int{1, 3}) {
		t.Fatalf("Ignore().Take(2) = %v", got)
	}
}

func TestTryR3TakeWhile(t *testing.T) {
	// R3: an errored element passes through TakeWhile without terminating;
	// only a successful element failing pred ends the sequence.
	got := collectTry(mixed().TakeWhile(func(v int) bool { return v < 3 }))
	want := []tv{{1, nil}, {0, e1}} // 3 fails pred; e1 passed through before it
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestTryR4ZeroOnError(t *testing.T) {
	// R4: operators that generate an error yield the zero value with it.
	t.Run("Seq.MapErr", func(t *testing.T) {
		got := collectTry(catena.Of(1, 2, 3).MapErr(func(v int) (int, error) {
			if v == 2 {
				return 999, e1 // returns junk WITH the error; operator must zero it
			}
			return v * 10, nil
		}))
		want := []tv{{10, nil}, {0, e1}, {30, nil}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Seq.FilterErr", func(t *testing.T) {
		got := collectTry(catena.Of(1, 2, 3).FilterErr(func(v int) (bool, error) {
			if v == 2 {
				return true, e1
			}
			return true, nil
		}))
		want := []tv{{1, nil}, {0, e1}, {3, nil}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Try.MapErr_callback_fails", func(t *testing.T) {
		got := collectTry(trySrc(tv{2, nil}).MapErr(func(int) (int, error) { return 999, e1 }))
		if !slices.Equal(got, []tv{{0, e1}}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Try.FilterErr_callback_fails", func(t *testing.T) {
		got := collectTry(trySrc(tv{2, nil}).FilterErr(func(int) (bool, error) { return true, e1 }))
		if !slices.Equal(got, []tv{{0, e1}}) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestTryR5FirstErrorTerminals(t *testing.T) {
	t.Run("Collect", func(t *testing.T) {
		vals, err := mixed().Collect()
		if !slices.Equal(vals, []int{1}) || err != e1 {
			t.Fatalf("got %v, %v", vals, err)
		}
		vals, err = trySrc(tv{1, nil}, tv{2, nil}).Collect()
		if !slices.Equal(vals, []int{1, 2}) || err != nil {
			t.Fatalf("clean: got %v, %v", vals, err)
		}
	})
	t.Run("Fold", func(t *testing.T) {
		acc, err := mixed().Fold(100, func(a, v int) int { return a + v })
		if acc != 101 || err != e1 {
			t.Fatalf("got %d, %v", acc, err)
		}
		acc, err = trySrc(tv{1, nil}, tv{2, nil}).Fold(0, func(a, v int) int { return a + v })
		if acc != 3 || err != nil {
			t.Fatalf("clean: got %d, %v", acc, err)
		}
	})
	t.Run("ForEach", func(t *testing.T) {
		var saw []int
		err := mixed().ForEach(func(v int) error { saw = append(saw, v); return nil })
		if err != e1 || !slices.Equal(saw, []int{1}) {
			t.Fatalf("element error: %v, saw %v", err, saw)
		}
		err = trySrc(tv{1, nil}, tv{2, nil}, tv{3, nil}).ForEach(func(v int) error {
			if v == 2 {
				return e2
			}
			return nil
		})
		if err != e2 {
			t.Fatalf("callback error: %v", err)
		}
		if err := trySrc(tv{1, nil}).ForEach(func(int) error { return nil }); err != nil {
			t.Fatalf("clean: %v", err)
		}
	})
	t.Run("Err", func(t *testing.T) {
		if err := mixed().Err(); err != e1 {
			t.Fatalf("got %v", err)
		}
		if err := trySrc(tv{1, nil}).Err(); err != nil {
			t.Fatalf("clean: %v", err)
		}
		// stops at the first error rather than draining
		consumed := 0
		counted := catena.FromErrs(func(yield func(int, error) bool) {
			for _, it := range []tv{{1, nil}, {0, e1}, {3, nil}} {
				consumed++
				if !yield(it.v, it.err) {
					return
				}
			}
		})
		counted.Err()
		if consumed != 2 {
			t.Fatalf("Err consumed %d elements, want 2", consumed)
		}
	})
	t.Run("Count", func(t *testing.T) {
		n, err := mixed().Count()
		if n != 1 || err != e1 {
			t.Fatalf("got %d, %v", n, err)
		}
		n, err = trySrc(tv{1, nil}, tv{2, nil}).Count()
		if n != 2 || err != nil {
			t.Fatalf("clean: got %d, %v", n, err)
		}
	})
}

func TestTryRecover(t *testing.T) {
	got := collectTry(mixed().Recover(func(err error) (int, bool) {
		if err == e1 {
			return 99, true // replace
		}
		return 0, false // pass through
	}))
	want := []tv{{1, nil}, {99, nil}, {3, nil}, {0, e2}, {5, nil}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestTryWrapErr(t *testing.T) {
	t.Run("wraps", func(t *testing.T) {
		got := collectTry(mixed().WrapErr(func(err error) error { return fmt.Errorf("row: %w", err) }))
		if got[1].err == e1 || !errors.Is(got[1].err, e1) {
			t.Fatalf("not wrapped: %v", got[1].err)
		}
		if got[0].err != nil || got[2].err != nil {
			t.Fatal("successes touched")
		}
	})
	t.Run("nil_return_keeps_original", func(t *testing.T) {
		got := collectTry(mixed().WrapErr(func(error) error { return nil }))
		if got[1].err != e1 {
			t.Fatalf("original error lost: %v", got[1].err)
		}
	})
}

func TestTryOnError(t *testing.T) {
	var saw []error
	_, errs := mixed().OnError(func(err error) { saw = append(saw, err) }).CollectAll()
	if !slices.Equal(saw, []error{e1, e2}) || !slices.Equal(errs, []error{e1, e2}) {
		t.Fatalf("saw %v, collected %v", saw, errs)
	}
}

func TestTryCollectAll(t *testing.T) {
	vals, errs := mixed().CollectAll()
	if !slices.Equal(vals, []int{1, 3, 5}) || !slices.Equal(errs, []error{e1, e2}) {
		t.Fatalf("got %v, %v", vals, errs)
	}
	vals, errs = trySrc(tv{1, nil}).CollectAll()
	if !slices.Equal(vals, []int{1}) || errs != nil {
		t.Fatalf("clean drain must return a nil error slice: %v, %v", vals, errs)
	}
	vals, errs = catena.EmptyTry[int]().CollectAll()
	if vals != nil || errs != nil {
		t.Fatalf("empty: %v, %v", vals, errs)
	}
}

func TestTryIgnoreAndErrs(t *testing.T) {
	if got := mixed().Ignore().Collect(); !slices.Equal(got, []int{1, 3, 5}) {
		t.Fatalf("Ignore: %v", got)
	}
	if got := mixed().Errs().Collect(); !slices.Equal(got, []error{e1, e2}) {
		t.Fatalf("Errs: %v", got)
	}
}

func TestTryMust(t *testing.T) {
	if got := trySrc(tv{1, nil}, tv{2, nil}).Must().Collect(); !slices.Equal(got, []int{1, 2}) {
		t.Fatalf("clean: %v", got)
	}
	recovered := mustPanic(t, func() { mixed().Must().Collect() })
	if recovered != e1 {
		t.Fatalf("panic value is %v, want the error itself", recovered)
	}
}

func TestTryUntilDone(t *testing.T) {
	t.Run("background_passthrough", func(t *testing.T) {
		got := collectTry(mixed().UntilDone(context.Background()))
		if !slices.Equal(got, collectTry(mixed())) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("cancelled_mid_stream", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		n := 0
		var got []tv
		for v, err := range trySrc(tv{1, nil}, tv{2, nil}, tv{3, nil}).UntilDone(ctx).Seq2() {
			got = append(got, tv{v, err})
			if n++; n == 1 {
				cancel()
			}
		}
		want := []tv{{1, nil}, {0, context.Canceled}}
		if !slices.Equal(got, want) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestSeqUntilDoneCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	n := 0
	var got []tv
	for v, err := range catena.Of(1, 2, 3).UntilDone(ctx).Seq2() {
		got = append(got, tv{v, err})
		if n++; n == 1 {
			cancel()
		}
	}
	want := []tv{{1, nil}, {0, context.Canceled}}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v", got)
	}
}

func TestTryTakeDropStateAndPanics(t *testing.T) {
	// L1 for the stateful Try operators: re-iteration over a re-iterable
	// source yields identical output.
	items := []tv{{1, nil}, {0, e1}, {3, nil}}
	reiterable := catena.FromErrs(func(yield func(int, error) bool) {
		for _, it := range items {
			if !yield(it.v, it.err) {
				return
			}
		}
	})
	taken := reiterable.Take(2)
	if a, b := collectTry(taken), collectTry(taken); !slices.Equal(a, b) {
		t.Fatalf("Try.Take captured state: %v then %v", a, b)
	}
	dropped := reiterable.Drop(1)
	if a, b := collectTry(dropped), collectTry(dropped); !slices.Equal(a, b) {
		t.Fatalf("Try.Drop captured state: %v then %v", a, b)
	}
	mustPanic(t, func() { reiterable.Take(-1) })
	mustPanic(t, func() { reiterable.Drop(-1) })
	if got := collectTry(reiterable.Take(0)); got != nil {
		t.Fatalf("Take(0): %v", got)
	}
	if got := collectTry(reiterable.Drop(0)); !slices.Equal(got, items) {
		t.Fatalf("Drop(0): %v", got)
	}
}

func TestTryNilReceivers(t *testing.T) {
	var n catena.Try[int]
	if got := collectTry(n.Map(func(v int) int { return v })); got != nil {
		t.Fatal("Map")
	}
	if got := collectTry(n.Filter(func(int) bool { return true })); got != nil {
		t.Fatal("Filter")
	}
	if got := collectTry(n.Take(3)); got != nil {
		t.Fatal("Take")
	}
	if got, err := n.Collect(); got != nil || err != nil {
		t.Fatal("Collect")
	}
	if got := n.Ignore().Collect(); got != nil {
		t.Fatal("Ignore")
	}
	if err := n.Err(); err != nil {
		t.Fatal("Err")
	}
	next, stop := n.Pull()
	defer stop()
	if _, _, ok := next(); ok {
		t.Fatal("Pull on nil yielded")
	}
}

func TestTryPull(t *testing.T) {
	next, stop := mixed().Pull()
	defer stop()
	v, err, ok := next()
	if v != 1 || err != nil || !ok {
		t.Fatalf("first: %v %v %v", v, err, ok)
	}
	v, err, ok = next()
	if v != 0 || err != e1 || !ok {
		t.Fatalf("second: %v %v %v", v, err, ok)
	}
}

func TestTryEarlyTermination(t *testing.T) {
	// Every Try intermediate must propagate a false yield immediately.
	inf := catena.FromErrs(func(yield func(int, error) bool) {
		for i := 0; ; i++ {
			var err error
			if i%3 == 1 {
				err = e1
			}
			if !yield(i, err) {
				return
			}
		}
	})
	ops := map[string]catena.Try[int]{
		"Map":       inf.Map(func(v int) int { return v }),
		"MapErr":    inf.MapErr(func(v int) (int, error) { return v, nil }),
		"FlatMap":   inf.FlatMap(func(v int) catena.Try[int] { return trySrc(tv{v, nil}) }),
		"Filter":    inf.Filter(func(int) bool { return true }),
		"FilterErr": inf.FilterErr(func(int) (bool, error) { return true, nil }),
		"Take":      inf.Take(1000),
		"TakeWhile": inf.TakeWhile(func(int) bool { return true }),
		"Drop":      inf.Drop(2),
		"OnEach":    inf.OnEach(func(int) {}),
		"OnError":   inf.OnError(func(error) {}),
		"Recover":   inf.Recover(func(error) (int, bool) { return 0, false }),
		"WrapErr":   inf.WrapErr(func(err error) error { return err }),
		"UntilDone": inf.UntilDone(context.Background()),
	}
	for name, op := range ops {
		t.Run(name, func(t *testing.T) {
			mustComplete(t, testTimeout, func() {
				n := 0
				for range op.Seq2() {
					if n++; n == 3 {
						break
					}
				}
			})
		})
	}
	t.Run("Ignore", func(t *testing.T) {
		mustComplete(t, testTimeout, func() { inf.Ignore().Take(3).Drain() })
	})
	t.Run("Errs", func(t *testing.T) {
		mustComplete(t, testTimeout, func() { inf.Errs().Take(3).Drain() })
	})
	t.Run("Must", func(t *testing.T) {
		clean := catena.FromErrs(func(yield func(int, error) bool) {
			for i := 0; ; i++ {
				if !yield(i, nil) {
					return
				}
			}
		})
		mustComplete(t, testTimeout, func() { clean.Must().Take(3).Drain() })
	})
}

// Branch closure: errored elements rejected by the consumer mid-yield.
func TestTryEarlyBreakBranches(t *testing.T) {
	t.Run("TryFilterErr_break_on_error_element", func(t *testing.T) {
		bad := trySrc(tv{1, nil}).FilterErr(func(v int) (bool, error) { return false, e1 })
		if got := collectTry(bad.Take(1)); !slices.Equal(got, []tv{{0, e1}}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("TryFlatMap_break_on_errored_input", func(t *testing.T) {
		src := trySrc(tv{0, e1}, tv{2, nil})
		got := collectTry(src.FlatMap(func(v int) catena.Try[int] { return trySrc(tv{v, nil}) }).Take(1))
		if !slices.Equal(got, []tv{{0, e1}}) {
			t.Fatalf("got %v", got)
		}
	})
}
