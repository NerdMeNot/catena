package parallel

import (
	"context"
	"sync"

	"github.com/NerdMeNot/catena"
)

// Map returns a stage that yields f applied to each element, running up to
// n calls at once and yielding results in source order. A failed call
// yields (zero, err) and the stage continues. When ctx is done the stage
// stops consuming, yields (zero, ctx.Err()) once in place of any results
// it still held, and ends; over an empty source it yields nothing. f
// receives a context that is also cancelled the moment the consumer
// stops. The
// source runs on the consumer's goroutine. ⚠ Starts up to n goroutines
// per iteration, all joined before it returns. ⚠ Consumes at
// most n elements beyond those yielded, and holds their results: one slow
// call at the head delays everything behind it (MapUnordered does not).
// ⚠ A finished result is delivered, and a done ctx is noticed, only when
// the source next yields or ends: behind a slow live source both wait
// for the next element.
// Panics if n <= 0.
func Map[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U] {
	return mapStage("Map", ctx, n, f, true)
}

// MapUnordered is Map yielding each result as soon as its call finishes
// rather than in source order: the same elements, bounds and error rules,
// with no call able to delay another. ⚠ Starts up to n goroutines per
// iteration, all joined before it returns. ⚠ Consumes at most n elements
// beyond those yielded. ⚠ A finished result is delivered only when the
// source next yields or ends. Panics if n <= 0.
func MapUnordered[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U] {
	return mapStage("MapUnordered", ctx, n, f, false)
}

// ForEach returns a terminal stage that calls f for each element, up to n
// calls at once, and returns the first error to occur — a call's own, or
// ctx.Err() — or nil once every call has succeeded. "First" is first in
// time, not in source order. A done ctx is an error even if every call
// that ran succeeded. On an error the remaining calls are cancelled
// through their context and joined before it returns. ⚠ Starts up to n goroutines, all joined before it returns.
// ⚠ Full drain on success. Panics if n <= 0.
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
			w := newWindow[U](n, ordered)

			// halt cancels the outstanding calls and joins them. It
			// takes their results rather than only waiting, because a
			// call may be blocked handing its result over; a panic
			// among them is kept for the end. Idempotent.
			var crash any
			halt := func() {
				cancel()
				for w.size > 0 {
					if r := w.take(); crash == nil {
						crash = r.crash
					}
				}
				wg.Wait()
			}
			defer halt() // joined on every exit, panics included

			var cause error // the parent context's error, once seen
			stopped := false

			// emit delivers the next result; false means stop.
			emit := func() bool {
				r := w.take()
				if r.crash != nil {
					halt() // join before the panic unwinds the source
					panic(r.crash)
				}
				// The parent, never cctx: the stage's own cancel
				// is not the caller's.
				if cause = ctx.Err(); cause != nil {
					return false
				}
				returned := false
				defer func() {
					if !returned {
						halt() // the consumer panicked: join before it unwinds the source
					}
				}()
				stopped = !yield(r.u, r.err)
				returned = true
				return !stopped
			}

			s.Seq()(func(v T) bool {
				if cause = ctx.Err(); cause != nil {
					halt()
					return false
				}
				out := w.reserve()
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
				// A full window must give up a result before another
				// element is taken; beyond that, hand over whatever
				// has already finished rather than hold it.
				for w.size == n || (w.size > 0 && w.ready()) {
					if !emit() {
						halt() // join before the source unwinds
						return false
					}
				}
				return true
			})
			for w.size > 0 && !stopped && cause == nil {
				emit()
			}
			halt()
			if crash != nil { // a call that panicked is never swallowed
				panic(crash)
			}
			if cause != nil {
				var zero U
				yield(zero, cause)
			}
		}
	}
}

// window holds the outstanding calls of one iteration: those started
// whose results the consumer has not yet taken. Only the consumer's
// goroutine touches it; a worker's whole contact is one send on the
// channel reserve gave it.
//
// In order, the next result is the oldest call's, so each call gets a
// channel of its own, in a queue: its one send always finds room.
// Unordered, it is whichever call finishes first, so they share one.
//
// Nothing here is sized by n. The queue grows with the calls actually
// outstanding, and the shared channel is capped, so a large n costs
// nothing until it is used. Beyond the cap a finished call waits to hand
// its result over — still one of the n in flight, as it would be anyway
// until the consumer took the result.
type window[U any] struct {
	ordered bool
	size    int                  // outstanding calls
	slots   fifo[chan result[U]] // ordered: one per outstanding call, oldest first
	spare   []chan result[U]     // ordered: channels to reuse
	shared  chan result[U]       // unordered
}

// sharedCap bounds the unordered results buffered ahead of the consumer.
const sharedCap = 256

func newWindow[U any](n int, ordered bool) *window[U] {
	w := &window[U]{ordered: ordered}
	if !ordered {
		w.shared = make(chan result[U], min(n, sharedCap))
	}
	return w
}

// reserve makes room for one more call and returns where it must send
// its result, exactly once.
func (w *window[U]) reserve() chan<- result[U] {
	w.size++
	if !w.ordered {
		return w.shared
	}
	var ch chan result[U]
	if n := len(w.spare); n > 0 {
		ch, w.spare = w.spare[n-1], w.spare[:n-1]
	} else {
		ch = make(chan result[U], 1)
	}
	w.slots.push(ch)
	return ch
}

// ready reports whether take would return without waiting.
func (w *window[U]) ready() bool {
	if w.ordered {
		return len(w.slots.peek()) > 0
	}
	return len(w.shared) > 0
}

// take returns the next result, waiting for it if it must. There must be
// an outstanding call.
func (w *window[U]) take() result[U] {
	w.size--
	if !w.ordered {
		return <-w.shared
	}
	ch := w.slots.pop()
	r := <-ch
	w.spare = append(w.spare, ch)
	return r
}

// fifo is a queue that reuses its storage: popping moves a cursor, and the
// live elements are moved down once the dead prefix is at least as long,
// so the backing array stays proportional to the most elements ever held
// at once rather than to the number ever pushed.
type fifo[E any] struct {
	items []E
	head  int
}

func (q *fifo[E]) len() int { return len(q.items) - q.head }

func (q *fifo[E]) push(e E) { q.items = append(q.items, e) }

func (q *fifo[E]) peek() E { return q.items[q.head] }

func (q *fifo[E]) pop() E {
	e := q.items[q.head]
	var zero E
	q.items[q.head] = zero // let go of what it held
	q.head++
	if q.head*2 >= len(q.items) {
		n := copy(q.items, q.items[q.head:])
		clear(q.items[n:])
		q.items, q.head = q.items[:n], 0
	}
	return e
}
