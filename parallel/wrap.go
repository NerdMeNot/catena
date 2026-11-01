package parallel

import (
	"context"
	"sync"
	"time"

	"github.com/NerdMeNot/catena"
)

// Retry returns f made to try again when it fails: up to attempts tries in
// all, waiting backoff(k) after failed try k (1, 2, …) before the next.
// It stops at the first success, after the last try, when retryable
// reports false for the error, or when ctx is done — a wait cut short by
// ctx returns the last try's error and makes no further try. A nil
// retryable retries every error, and a nil backoff never waits. A panic in
// f is not retried. What it returns has the shape of f, so it goes
// anywhere f does:
//
//	parallel.Map(ctx, 8, parallel.Retry(3, backoff, isTransient, fetch))
//
// ⚠ Starts no goroutines: it waits on the goroutine that called it, for
// up to the sum of the backoffs. Panics if attempts <= 0.
func Retry[T, U any](attempts int, backoff func(attempt int) time.Duration, retryable func(error) bool, f func(context.Context, T) (U, error)) func(context.Context, T) (U, error) {
	posCheck("Retry", "attempts", attempts)
	return func(ctx context.Context, v T) (U, error) {
		for attempt := 1; ; attempt++ {
			u, err := f(ctx, v)
			if err == nil || attempt == attempts || ctx.Err() != nil ||
				(retryable != nil && !retryable(err)) {
				return u, err
			}
			if backoff == nil {
				continue
			}
			if d := backoff(attempt); d > 0 {
				t := time.NewTimer(d)
				select {
				case <-t.C:
				case <-ctx.Done():
					t.Stop()
					return u, err
				}
			}
		}
	}
}

// Limit returns f made to keep to a rate: at most n calls are let through
// to f in any period of length per — 50 a second is Limit(50,
// time.Second, f). A call over the limit waits for its turn; if ctx is
// done first it returns (zero, ctx.Err()) without calling f, and the turn
// it was given is spent. Calls take their turns in the order they arrive.
// The rate is kept when calls are let through: a goroutine the scheduler
// is slow to resume can reach f later than its turn, as with any limiter
// on the calling side, so leave a server's limit some headroom.
//
// The limit is shared by every call of the function it returns, across
// stages and iterations: make one per quota and pass it wherever that
// quota applies. Wrapped around f inside Retry, every try counts against
// it:
//
//	parallel.Retry(3, backoff, isTransient, parallel.Limit(50, time.Second, fetch))
//
// ⚠ Starts no goroutines: a call waits on the goroutine that made it. ⚠
// Holds the start times of the last n calls. Panics if n <= 0 or per <= 0.
func Limit[T, U any](n int, per time.Duration, f func(context.Context, T) (U, error)) func(context.Context, T) (U, error) {
	posCheck("Limit", "n", n)
	posCheck("Limit", "per", per)
	var mu sync.Mutex
	// The start times given out so far, the last n of them, oldest at
	// next. They never decrease, so a call may start once the call n
	// turns before it is a full period in the past.
	starts := make([]time.Time, 0, n)
	next := 0
	return func(ctx context.Context, v T) (U, error) {
		mu.Lock()
		now := time.Now()
		at := now
		if len(starts) < n {
			starts = append(starts, at)
		} else {
			if t := starts[next].Add(per); t.After(at) {
				at = t
			}
			starts[next] = at
			next = (next + 1) % n
		}
		mu.Unlock()
		if wait := at.Sub(now); wait > 0 {
			t := time.NewTimer(wait)
			select {
			case <-t.C:
			case <-ctx.Done():
				t.Stop()
				var zero U
				return zero, ctx.Err()
			}
		}
		return f(ctx, v)
	}
}

// Do runs every function at once and returns the first error to occur, or
// nil once all have succeeded: ForEach for calls that are each different
// rather than one call per element — the three loads a page needs, say.
// Each function receives a context that is cancelled when another fails
// or ctx is done, and every one has returned before Do does, so whatever
// they wrote can be read safely after it. A done ctx is an error even if
// every function succeeded. With no functions it returns nil. ⚠ Starts
// one goroutine per function, all joined before it returns.
func Do(ctx context.Context, fns ...func(context.Context) error) error {
	if len(fns) == 0 {
		return nil
	}
	return catena.FromSlice(fns).Then(ForEach(ctx, len(fns), func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	}))
}
