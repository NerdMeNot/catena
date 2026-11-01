package parallel

import (
	"context"
	"errors"
	"math/rand/v2"
	"strconv"
	"sync"
	"time"

	"github.com/NerdMeNot/catena"
)

// Retry returns f made to try again when it fails: up to attempts tries in
// all, waiting backoff(k, err) after failed try k (1, 2, …), err being
// that try's error, before the next. It stops at the first success, after
// the last try, or when retryable reports false for the error, and returns
// that try's result. A nil retryable retries every error; a nil backoff
// never waits, and Backoff makes the usual one. Having the error, a
// backoff can honour a server's own advice, such as a Retry-After header
// the error carries.
//
// It also stops when ctx is done, between tries or during a wait, and
// then returns the last try's error joined with ctx.Err(), so that
// errors.Is finds either. A panic in f is not retried. What it returns
// has the shape of f, so it goes anywhere f does, such as the callback of
// Map. ⚠ Starts no goroutines: it waits on the goroutine that called it,
// for up to the sum of the backoffs. Panics if attempts <= 0 or f is nil.
func Retry[T, U any](attempts int, backoff func(attempt int, err error) time.Duration, retryable func(error) bool, f func(context.Context, T) (U, error)) func(context.Context, T) (U, error) {
	posCheck("Retry", "attempts", attempts)
	nilCheck("Retry", "f", f == nil)
	return func(ctx context.Context, v T) (U, error) {
		for attempt := 1; ; attempt++ {
			u, err := f(ctx, v)
			if err == nil || attempt == attempts || (retryable != nil && !retryable(err)) {
				return u, err
			}
			if ctx.Err() != nil {
				return u, withCause(err, ctx)
			}
			if backoff == nil {
				continue
			}
			if d := backoff(attempt, err); d > 0 {
				t := time.NewTimer(d)
				select {
				case <-t.C:
				case <-ctx.Done():
					t.Stop()
					return u, withCause(err, ctx)
				}
			}
		}
	}
}

// withCause joins a failed try's error with the done context's, unless the
// try already failed because of it.
func withCause(err error, ctx context.Context) error {
	if errors.Is(err, ctx.Err()) {
		return err
	}
	return errors.Join(err, ctx.Err())
}

// Backoff returns a backoff for Retry that grows exponentially: a wait of
// up to base after the first failed try, doubling after each one after
// it, never more than ceiling. Each wait is drawn at random from the upper
// half of that — between 50 and 100ms, where it is 100ms — so that callers
// that failed together do not all try again together. It ignores the
// error, so a backoff that honours one — a Retry-After it carries — can
// fall back to it when there is none. ⚠ Starts nothing and holds nothing; the randomness is math/rand/v2's.
// Panics if base <= 0 or ceiling < base.
func Backoff(base, ceiling time.Duration) func(attempt int, err error) time.Duration {
	posCheck("Backoff", "base", base)
	if ceiling < base {
		panic("parallel: Backoff: ceiling must be >= base")
	}
	return func(attempt int, _ error) time.Duration {
		d := base
		for i := 1; i < attempt && d < ceiling; i++ {
			d *= 2
		}
		d = min(d, ceiling)
		return d - rand.N(d/2+1)
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
// quota applies. Wrapped around f inside Retry — Retry(…, Limit(…, f)) —
// every try counts against it. ⚠ Starts no goroutines: a call waits on the
// goroutine that made it. ⚠ Holds the start times of the last n calls.
// Panics if n <= 0, per <= 0 or f is nil.
func Limit[T, U any](n int, per time.Duration, f func(context.Context, T) (U, error)) func(context.Context, T) (U, error) {
	posCheck("Limit", "n", n)
	posCheck("Limit", "per", per)
	nilCheck("Limit", "f", f == nil)
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
// every function succeeded, and a ctx already done when Do is called
// returns its error without calling any. With no functions it returns
// nil. ⚠ Starts one goroutine per function, all joined before it returns.
// Panics, before calling any, if a function is nil.
func Do(ctx context.Context, fns ...func(context.Context) error) error {
	for i, fn := range fns {
		nilCheck("Do", "fns["+strconv.Itoa(i)+"]", fn == nil)
	}
	if len(fns) == 0 {
		return nil
	}
	return catena.FromSlice(fns).Then(ForEach(ctx, len(fns), func(ctx context.Context, fn func(context.Context) error) error {
		return fn(ctx)
	}))
}
