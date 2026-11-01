package parallel_test

// Retry, Limit and Do: the callback wrappers and the run-these-at-once
// call. Every timing here is virtual, so it is asserted exactly.

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/parallel"
)

// flaky fails its first `fails` calls with errBoom, recording when each
// call began, then succeeds.
type flaky struct {
	fails int
	mu    sync.Mutex
	at    []time.Duration
	start time.Time
}

func (f *flaky) call(_ context.Context, v int) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.at = append(f.at, time.Since(f.start))
	if len(f.at) <= f.fails {
		return 0, errBoom
	}
	return v * 2, nil
}

func linear(attempt int, _ error) time.Duration {
	return time.Duration(attempt) * 100 * time.Millisecond
}

func TestRetry(t *testing.T) {
	ctx := context.Background()

	bubble(t, "succeeds_after_failures", func(t *testing.T) {
		f := &flaky{fails: 2, start: time.Now()}
		v, err := parallel.Retry(5, linear, nil, f.call)(ctx, 21)
		// Tries at 0, then 100ms after the first, then 200ms after the second.
		want := []time.Duration{0, 100 * time.Millisecond, 300 * time.Millisecond}
		if v != 42 || err != nil || !slices.Equal(f.at, want) {
			t.Fatalf("v=%d err=%v tries at %v", v, err, f.at)
		}
	})

	bubble(t, "gives_up_after_the_last_try_without_waiting", func(t *testing.T) {
		f := &flaky{fails: 10, start: time.Now()}
		_, err := parallel.Retry(3, linear, nil, f.call)(ctx, 1)
		if !errors.Is(err, errBoom) || len(f.at) != 3 || time.Since(f.start) != 300*time.Millisecond {
			t.Fatalf("err=%v tries=%d took=%v", err, len(f.at), time.Since(f.start))
		}
	})

	bubble(t, "an_error_not_worth_retrying_returns_at_once", func(t *testing.T) {
		f := &flaky{fails: 10, start: time.Now()}
		never := func(error) bool { return false }
		if _, err := parallel.Retry(5, linear, never, f.call)(ctx, 1); !errors.Is(err, errBoom) || len(f.at) != 1 {
			t.Fatalf("err=%v tries=%d", err, len(f.at))
		}
	})

	bubble(t, "no_backoff_and_zero_backoff_never_wait", func(t *testing.T) {
		for _, backoff := range []func(int, error) time.Duration{nil, func(int, error) time.Duration { return 0 }} {
			f := &flaky{fails: 2, start: time.Now()}
			if v, err := parallel.Retry(3, backoff, nil, f.call)(ctx, 1); v != 2 || err != nil || time.Since(f.start) != 0 {
				t.Fatalf("v=%d err=%v took=%v", v, err, time.Since(f.start))
			}
		}
	})

	bubble(t, "a_cancelled_wait_returns_the_last_error", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(150*time.Millisecond, cancel)
		f := &flaky{fails: 10, start: time.Now()}
		_, err := parallel.Retry(5, linear, nil, f.call)(cctx, 1)
		// Tries at 0 and 100ms; the wait for the third is cut at 150ms,
		// and both the try's error and the context's are reported.
		if !errors.Is(err, errBoom) || !errors.Is(err, context.Canceled) ||
			len(f.at) != 2 || time.Since(f.start) != 150*time.Millisecond {
			t.Fatalf("err=%v tries=%d took=%v", err, len(f.at), time.Since(f.start))
		}
	})

	bubble(t, "a_done_context_is_not_retried", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		f := &flaky{fails: 10, start: time.Now()}
		_, err := parallel.Retry(5, linear, nil, f.call)(cctx, 1)
		if !errors.Is(err, errBoom) || !errors.Is(err, context.Canceled) || len(f.at) != 1 {
			t.Fatalf("err=%v tries=%d", err, len(f.at))
		}
	})

	bubble(t, "a_try_that_failed_from_the_context_is_not_reported_twice", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := parallel.Retry(5, linear, nil, func(ctx context.Context, _ int) (int, error) {
			return 0, ctx.Err()
		})(cctx, 1)
		if err != context.Canceled {
			t.Fatalf("err=%#v", err)
		}
	})

	bubble(t, "the_backoff_sees_the_error", func(t *testing.T) {
		// A server that says how long to wait: the backoff honours it.
		var seen []error
		calls := 0
		start := time.Now()
		_, err := parallel.Retry(3, func(n int, err error) time.Duration {
			seen = append(seen, err)
			var ra *retryAfter
			errors.As(err, &ra)
			return ra.d
		}, nil, func(context.Context, int) (int, error) {
			calls++
			if calls < 3 {
				return 0, &retryAfter{time.Duration(calls) * time.Second}
			}
			return 7, nil
		})(ctx, 1)
		if err != nil || len(seen) != 2 || time.Since(start) != 3*time.Second {
			t.Fatalf("err=%v seen=%v took=%v", err, seen, time.Since(start))
		}
	})

	bubble(t, "a_panic_is_not_retried", func(t *testing.T) {
		tries := 0
		retried := parallel.Retry(5, nil, nil, func(context.Context, int) (int, error) {
			tries++
			panic("boom")
		})
		if v := mustPanic(t, func() { retried(ctx, 1) }); v != "boom" || tries != 1 {
			t.Fatalf("panic %v after %d tries", v, tries)
		}
	})

	bubble(t, "inside_a_stage", func(t *testing.T) {
		var mu sync.Mutex
		tries := map[int]int{}
		f := func(_ context.Context, v int) (int, error) {
			mu.Lock()
			defer mu.Unlock()
			if tries[v]++; tries[v] < 3 {
				return 0, errBoom
			}
			return v, nil
		}
		got, err := catena.Range(0, 20, 1).
			Then(parallel.Map(ctx, 4, parallel.Retry(3, linear, nil, f))).
			Collect()
		if err != nil || !slices.Equal(got, catena.Range(0, 20, 1).Collect()) {
			t.Fatalf("got %v err %v", got, err)
		}
	})

	t.Run("bad_arguments_panic_at_construction", func(t *testing.T) {
		if v := mustPanic(t, func() { parallel.Retry(0, nil, nil, scrambled) }); v != "parallel: Retry: attempts must be > 0" {
			t.Fatalf("panic %v", v)
		}
		if v := mustPanic(t, func() { parallel.Retry[int, int](3, nil, nil, nil) }); v != "parallel: Retry: f is nil" {
			t.Fatalf("panic %v", v)
		}
	})
}

func TestLimit(t *testing.T) {
	ctx := context.Background()

	bubble(t, "keeps_to_the_rate_across_workers", func(t *testing.T) {
		start := time.Now()
		var mu sync.Mutex
		var at []time.Duration
		record := func(_ context.Context, v int) (int, error) {
			mu.Lock()
			at = append(at, time.Since(start))
			mu.Unlock()
			return v, nil
		}
		got, err := catena.Range(0, 120, 1).
			Then(parallel.MapUnordered(ctx, 8, parallel.Limit(50, time.Second, record))).
			Collect()
		if err != nil || len(got) != 120 {
			t.Fatalf("got %d results, err %v", len(got), err)
		}
		// 50 at once, 50 a second later, the last 20 a second after that.
		if took := time.Since(start); took != 2*time.Second {
			t.Fatalf("took %v", took)
		}
		slices.Sort(at)
		for i := range len(at) - 50 {
			if at[i+50]-at[i] < time.Second {
				t.Fatalf("calls %d and %d started %v apart", i, i+50, at[i+50]-at[i])
			}
		}
	})

	bubble(t, "the_limit_is_shared_by_every_use", func(t *testing.T) {
		limited := parallel.Limit(2, time.Second, func(_ context.Context, v int) (int, error) { return v, nil })
		start := time.Now()
		for range 3 {
			catena.Of(1, 2).Then(parallel.Map(ctx, 2, limited)).Collect()
		}
		// Two calls a second, six calls: the third pair starts at 2s.
		if took := time.Since(start); took != 2*time.Second {
			t.Fatalf("took %v", took)
		}
	})

	bubble(t, "a_cancelled_wait_never_calls", func(t *testing.T) {
		var calls atomic.Int64
		limited := parallel.Limit(1, time.Second, func(_ context.Context, v int) (int, error) {
			calls.Add(1)
			return v, nil
		})
		limited(ctx, 1) // takes the only turn this second
		cctx, cancel := context.WithCancel(ctx)
		time.AfterFunc(300*time.Millisecond, cancel)
		start := time.Now()
		_, err := limited(cctx, 2)
		if !errors.Is(err, context.Canceled) || calls.Load() != 1 || time.Since(start) != 300*time.Millisecond {
			t.Fatalf("err=%v calls=%d waited=%v", err, calls.Load(), time.Since(start))
		}
	})

	t.Run("arguments_must_be_positive", func(t *testing.T) {
		if v := mustPanic(t, func() { parallel.Limit(0, time.Second, scrambled) }); v != "parallel: Limit: n must be > 0" {
			t.Fatalf("panic %v", v)
		}
		if v := mustPanic(t, func() { parallel.Limit(1, 0, scrambled) }); v != "parallel: Limit: per must be > 0" {
			t.Fatalf("panic %v", v)
		}
		if v := mustPanic(t, func() { parallel.Limit[int, int](1, time.Second, nil) }); v != "parallel: Limit: f is nil" {
			t.Fatalf("panic %v", v)
		}
	})
}

func TestDo(t *testing.T) {
	ctx := context.Background()

	t.Run("no_functions", func(t *testing.T) {
		if err := parallel.Do(ctx); err != nil {
			t.Fatalf("err=%v", err)
		}
	})

	bubble(t, "all_at_once", func(t *testing.T) {
		start := time.Now()
		var a, b, c int
		err := parallel.Do(ctx,
			func(context.Context) error { time.Sleep(time.Second); a = 1; return nil },
			func(context.Context) error { time.Sleep(2 * time.Second); b = 2; return nil },
			func(context.Context) error { time.Sleep(time.Second); c = 3; return nil },
		)
		if err != nil || a+b+c != 6 || time.Since(start) != 2*time.Second {
			t.Fatalf("err=%v a,b,c=%d,%d,%d took=%v", err, a, b, c, time.Since(start))
		}
	})

	bubble(t, "first_error_cancels_the_rest", func(t *testing.T) {
		var cancelled atomic.Int64
		wait := func(ctx context.Context) error {
			<-ctx.Done()
			cancelled.Add(1)
			return ctx.Err()
		}
		err := parallel.Do(ctx, wait, func(context.Context) error { return errBoom }, wait)
		// Both have returned by now: Do joins before it returns.
		if !errors.Is(err, errBoom) || cancelled.Load() != 2 {
			t.Fatalf("err=%v cancelled=%d", err, cancelled.Load())
		}
	})

	bubble(t, "a_panic_is_re_raised", func(t *testing.T) {
		v := mustPanic(t, func() {
			parallel.Do(ctx, func(context.Context) error { panic("load boom") })
		})
		if v != "load boom" {
			t.Fatalf("panic %v", v)
		}
	})

	t.Run("a_nil_function_panics_before_any_is_called", func(t *testing.T) {
		called := false
		v := mustPanic(t, func() {
			parallel.Do(ctx, func(context.Context) error { called = true; return nil }, nil)
		})
		if v != "parallel: Do: fns[1] is nil" || called {
			t.Fatalf("panic %v, called %v", v, called)
		}
	})

	bubble(t, "a_done_context_calls_nothing", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		called := false
		err := parallel.Do(cctx, func(context.Context) error { called = true; return nil })
		if !errors.Is(err, context.Canceled) || called {
			t.Fatalf("err=%v called=%v", err, called)
		}
	})
}

// retryAfter is an error that says how long to wait before trying again.
type retryAfter struct{ d time.Duration }

func (e *retryAfter) Error() string { return "retry after " + e.d.String() }

func TestStagesRefuseANilCallback(t *testing.T) {
	ctx := context.Background()
	for name, build := range map[string]func(){
		"Map":             func() { parallel.Map[int, int](ctx, 1, nil) },
		"MapUnordered":    func() { parallel.MapUnordered[int, int](ctx, 1, nil) },
		"MapTry":          func() { parallel.MapTry[int, int](ctx, 1, nil) },
		"MapUnorderedTry": func() { parallel.MapUnorderedTry[int, int](ctx, 1, nil) },
		"ForEach":         func() { parallel.ForEach[int](ctx, 1, nil) },
		"ForEachTry":      func() { parallel.ForEachTry[int](ctx, 1, nil) },
	} {
		if v := mustPanic(t, build); v != "parallel: "+name+": f is nil" {
			t.Errorf("%s: panic %v", name, v)
		}
	}
}

func TestBackoff(t *testing.T) {
	b := parallel.Backoff(100*time.Millisecond, time.Second)
	// Each wait is in the upper half of base·2^(n-1), capped at the
	// ceiling: 50–100ms, 100–200ms, 200–400ms, 400–800ms, then 500ms–1s.
	for n, hi := range map[int]time.Duration{
		1: 100 * time.Millisecond, 2: 200 * time.Millisecond, 3: 400 * time.Millisecond,
		4: 800 * time.Millisecond, 5: time.Second, 60: time.Second, 1000: time.Second,
	} {
		for range 200 {
			if d := b(n, errBoom); d < hi/2 || d > hi {
				t.Fatalf("attempt %d: %v outside [%v, %v]", n, d, hi/2, hi)
			}
		}
	}
	// The draws differ: callers that failed together spread out.
	seen := map[time.Duration]bool{}
	for range 50 {
		seen[b(3, nil)] = true
	}
	if len(seen) < 10 {
		t.Fatalf("only %d distinct waits in 50 draws", len(seen))
	}
	if v := mustPanic(t, func() { parallel.Backoff(0, time.Second) }); v != "parallel: Backoff: base must be > 0" {
		t.Fatalf("panic %v", v)
	}
	if v := mustPanic(t, func() { parallel.Backoff(time.Second, time.Millisecond) }); v != "parallel: Backoff: ceiling must be >= base" {
		t.Fatalf("panic %v", v)
	}
}
