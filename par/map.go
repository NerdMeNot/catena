package par

import (
	"context"
	"sync"

	"github.com/NerdMeNot/catena"
)

// Map returns a stage that yields f applied to each element, running up to
// n calls at once and yielding results in source order. A failed call
// yields (zero, err) and the stage continues. When ctx is done the stage
// stops consuming, yields (zero, ctx.Err()) once, and ends; f receives a
// context that is also cancelled the moment the consumer stops. Caller-
// driven: the source runs on the consumer's goroutine. ⚠ Starts up to n
// goroutines per iteration, all joined before it returns. Consumes at most
// n elements beyond those yielded, and holds their results: one slow call
// at the head delays everything behind it (MapUnordered does not). A
// finished result is delivered only when the source next yields or ends,
// so behind a slow live source it waits for the next element. Panics if
// n <= 0.
func Map[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U] {
	return mapStage("Map", ctx, n, f, true)
}

// MapUnordered is Map yielding each result as soon as its call finishes
// rather than in source order: the same elements, bounds and error rules,
// with no call able to delay another. ⚠ Starts up to n goroutines per
// iteration, all joined before it returns. Consumes at most n elements
// beyond those yielded. A finished result is delivered only when the
// source next yields or ends. Panics if n <= 0.
func MapUnordered[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U] {
	return mapStage("MapUnordered", ctx, n, f, false)
}

// ForEach returns a terminal stage that calls f for each element, up to n
// calls at once, and returns the first error to occur — a call's own, or
// ctx.Err() — or nil once every call has succeeded. On an error the
// remaining calls are cancelled through their context and joined before
// it returns. ⚠ Starts up to n goroutines, all joined before it returns.
// Full drain on success. Panics if n <= 0.
func ForEach[T any](ctx context.Context, n int, f func(context.Context, T) error) func(catena.Seq[T]) error {
	stage := mapStage("ForEach", ctx, n, func(ctx context.Context, v T) (struct{}, error) {
		return struct{}{}, f(ctx, v)
	}, false)
	return func(s catena.Seq[T]) error {
		return stage(s).Err()
	}
}

// result is what one call of the callback produced.
type result[U any] struct {
	u     U
	err   error
	crash any // what to re-raise if the call panicked or exited
}

// mapStage is the shared body of the caller-driven stages. op names the
// caller in the construction panic.
//
// The source is invoked directly on the consumer's goroutine, as every
// core operator invokes its source; only f runs elsewhere. Each element
// starts one goroutine, and the window of n outstanding calls is the only
// thing that ever blocks the consumer: when it is full, the consumer
// waits for a result and yields it before taking another element.
func mapStage[T, U any](op string, ctx context.Context, n int, f func(context.Context, T) (U, error), ordered bool) func(catena.Seq[T]) catena.Try[U] {
	posCheck(op, "n", n)
	return func(s catena.Seq[T]) catena.Try[U] {
		return func(yield func(U, error) bool) {
			cctx, cancel := context.WithCancel(ctx)
			var wg sync.WaitGroup
			halt := func() { cancel(); wg.Wait() } // idempotent
			defer halt()                           // joined on every exit, panics included

			// slots is a ring of the outstanding calls, oldest at head:
			// slots[(head+i)%n] is where the i-th of them delivers. In
			// order, each has its own channel, so the head is always
			// the oldest call. Unordered, they all share one, so the
			// head is whichever call finishes first. Either way a
			// worker's one send finds room and never blocks.
			slots := make([]chan result[U], n)
			if ordered {
				for i := range slots {
					slots[i] = make(chan result[U], 1)
				}
			} else {
				shared := make(chan result[U], n)
				for i := range slots {
					slots[i] = shared
				}
			}
			head, size := 0, 0
			take := func() result[U] {
				r := <-slots[head]
				head, size = (head+1)%n, size-1
				return r
			}

			var cause error // the parent context's error, once seen
			stopped := false

			// emit delivers the head result; false means stop.
			emit := func() bool {
				r := take()
				if r.crash != nil {
					halt() // join before the panic unwinds the source
					panic(r.crash)
				}
				// The parent, never cctx: the stage's own cancel
				// is not the caller's.
				if cause = ctx.Err(); cause != nil {
					return false
				}
				if !yield(r.u, r.err) {
					stopped = true
					return false
				}
				return true
			}

			s.Seq()(func(v T) bool {
				if cause = ctx.Err(); cause != nil {
					halt()
					return false
				}
				out := slots[(head+size)%n]
				size++
				wg.Go(func() {
					var r result[U]
					defer func() { out <- r }() // on return, panic and Goexit
					protect(&r.crash, func() {
						u, err := f(cctx, v)
						if err == nil {
							r.u = u // a failed call yields the zero value
						}
						r.err = err
					})
				})
				// A full window must give up its head before another
				// element is taken; beyond that, hand over whatever
				// has already finished rather than hold it.
				for size == n || (size > 0 && len(slots[head]) > 0) {
					if !emit() {
						halt() // join before the source unwinds
						return false
					}
				}
				return true
			})
			for size > 0 && !stopped && cause == nil {
				emit()
			}
			halt()
			for size > 0 { // a call that panicked is never swallowed
				if r := take(); r.crash != nil {
					panic(r.crash)
				}
			}
			if cause != nil {
				var zero U
				yield(zero, cause)
			}
		}
	}
}
