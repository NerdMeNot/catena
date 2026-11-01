package parallel_test

// The Try stages: what MapTry, MapUnorderedTry and ForEachTry do with a
// source that already carries errors. Everything else about them — order,
// bounds, cancellation, joining, panics — is the conformance harness's,
// which runs them over a Try with no errors in it.

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/parallel"
)

var errBad = errors.New("bad input")

// withErrs is a Try in which every negative value is an error.
func withErrs(vs ...int) catena.Try[int] {
	return catena.FromSlice(vs).MapErr(func(v int) (int, error) {
		if v < 0 {
			return 0, errBad
		}
		return v, nil
	})
}

type tryStage = func(context.Context, int, callback) func(catena.Try[int]) catena.Try[int]

var tryStages = []struct {
	name    string
	build   tryStage
	ordered bool
}{
	{"MapTry", parallel.MapTry[int, int], true},
	{"MapUnorderedTry", parallel.MapUnorderedTry[int, int], false},
}

func TestTryStagesPassErrorsThrough(t *testing.T) {
	ctx := context.Background()
	in := []int{3, -1, 5, 6, -1, -1, 9, 1, -1}
	var want []pair
	for _, v := range in {
		if v < 0 {
			want = append(want, pair{0, true})
		} else {
			want = append(want, modelScrambled([]int{v})...)
		}
	}
	for _, c := range tryStages {
		for _, n := range []int{1, 2, 3, 8} {
			bubble(t, c.name, func(t *testing.T) {
				var p probe
				got := collectPairs(c.build(ctx, n, p.wrap(scrambled))(withErrs(in...)))
				if !p.joined() {
					t.Fatal("calls still running after the iteration returned")
				}
				w := want
				if !c.ordered {
					got, w = sortPairs(got), sortPairs(want)
				}
				if !slices.Equal(got, w) {
					t.Fatalf("n=%d: got %v, want %v", n, got, w)
				}
				// f is never called on an errored element.
				if calls := p.started.Load(); calls != 5 {
					t.Fatalf("n=%d: %d calls for 5 good elements", n, calls)
				}
			})
		}
	}
}

func TestTryStagesErrorsAndTheWindow(t *testing.T) {
	// One slow call, an error, another slow call, with n = 2. In order,
	// the error holds its place in the window, so the window is full and
	// the second call cannot start until the first has been delivered.
	// Out of order, the error is yielded at once and takes no place, so
	// both calls run together.
	ctx := context.Background()
	slow := func(_ context.Context, v int) (int, error) {
		time.Sleep(time.Second)
		return v, nil
	}
	for _, c := range []struct {
		name  string
		build tryStage
		took  time.Duration
	}{
		{"MapTry", parallel.MapTry[int, int], 2 * time.Second},
		{"MapUnorderedTry", parallel.MapUnorderedTry[int, int], time.Second},
	} {
		bubble(t, c.name, func(t *testing.T) {
			start := time.Now()
			vs, errs := c.build(ctx, 2, slow)(withErrs(1, -1, 2)).CollectAll()
			if took := time.Since(start); took != c.took || len(vs) != 2 || len(errs) != 1 {
				t.Fatalf("took %v (want %v), %v %v", took, c.took, vs, errs)
			}
		})
	}
}

func TestTryStagesStopAtAnError(t *testing.T) {
	// The consumer stopping at a passed-through error ends the stage like
	// stopping anywhere else: calls cancelled and joined, source cleaned
	// up.
	ctx := context.Background()
	for _, c := range tryStages {
		bubble(t, c.name, func(t *testing.T) {
			var p probe
			src := source{n: -1}
			in := src.seq().MapErr(func(v int) (int, error) {
				if v == 3 {
					return 0, errBad
				}
				return v, nil
			})
			// The calls before the error finish, so that in order it can
			// be reached; those after it are still running when it is.
			f := p.wrap(func(ctx context.Context, v int) (int, error) {
				select {
				case <-time.After(time.Duration(v+1) * time.Second):
					return v, nil
				case <-ctx.Done():
					return 0, ctx.Err()
				}
			})
			if err := c.build(ctx, 8, f)(in).Err(); !errors.Is(err, errBad) {
				t.Fatalf("err=%v", err)
			}
			if !p.joined() || !src.clean() || p.cancelled.Load() == 0 {
				t.Fatalf("joined=%v clean=%v cancelled=%d", p.joined(), src.clean(), p.cancelled.Load())
			}
		})
	}
}

func TestForEachTry(t *testing.T) {
	ctx := context.Background()

	bubble(t, "all_succeed", func(t *testing.T) {
		var p probe
		f := p.wrap(scrambled)
		err := withErrs(1, 2, 3, 5).Then(parallel.ForEachTry(ctx, 2, func(ctx context.Context, v int) error {
			_, err := f(ctx, v)
			return err
		}))
		if err != nil || p.started.Load() != 4 || !p.joined() {
			t.Fatalf("err=%v started=%d joined=%v", err, p.started.Load(), p.joined())
		}
	})

	bubble(t, "an_errored_element_cancels_the_rest", func(t *testing.T) {
		var p probe
		f := p.wrap(func(ctx context.Context, v int) (int, error) {
			<-ctx.Done()
			return 0, ctx.Err()
		})
		err := withErrs(1, 2, -1, 4).Then(parallel.ForEachTry(ctx, 4, func(ctx context.Context, v int) error {
			_, err := f(ctx, v)
			return err
		}))
		// The source's error, not the cancellations it caused.
		if !errors.Is(err, errBad) || !p.joined() || p.started.Load() != 2 {
			t.Fatalf("err=%v joined=%v started=%d", err, p.joined(), p.started.Load())
		}
	})

	bubble(t, "nil_source", func(t *testing.T) {
		var none catena.Try[int]
		if err := none.Then(parallel.ForEachTry(ctx, 2, func(context.Context, int) error { return nil })); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	t.Run("n_must_be_positive", func(t *testing.T) {
		v := mustPanic(t, func() { parallel.ForEachTry(ctx, 0, func(context.Context, int) error { return nil }) })
		if v != "parallel: ForEachTry: n must be > 0" {
			t.Fatalf("panic %v", v)
		}
	})
}
