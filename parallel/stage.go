package parallel

import (
	"sync"
	"time"

	"github.com/NerdMeNot/catena"
)

// Buffer returns a stage that runs its source ahead of the consumer, on a
// goroutine of its own, holding up to n elements ready: a slow source and
// a slow consumer overlap instead of taking turns. Order is preserved.
// T cannot be inferred — write parallel.Buffer[int](64). ⚠ Starts one
// goroutine per iteration, joined before it returns, so stopping waits
// for the source's current step. ⚠ Buffers n elements and consumes up to
// n+1 beyond those yielded. ⚠ Everything upstream runs concurrently with
// the consumer's loop body. Panics if n <= 0.
func Buffer[T any](n int) func(catena.Seq[T]) catena.Seq[T] {
	posCheck("Buffer", "n", n)
	return func(s catena.Seq[T]) catena.Seq[T] {
		return func(yield func(T) bool) {
			f := start(n, s)
			defer f.stop() // if the consumer panics
			for v := range f.ch {
				if !yield(v) {
					break
				}
			}
			f.finish()
		}
	}
}

// Merge yields the elements of every sequence as they arrive, each
// sequence running on a goroutine of its own. Elements of one sequence
// keep their order; the interleaving between sequences is whatever the
// timing makes it. It ends when all have ended. ⚠ Starts one goroutine per
// sequence, plus one, per iteration, all joined before it returns, so
// stopping waits for every source's current step. ⚠ Buffers up to 64
// elements between the sources and the consumer, so together they run
// that far ahead, plus one each. ⚠ The sources run concurrently with each
// other and with the consumer's loop body.
func Merge[T any](seqs ...catena.Seq[T]) catena.Seq[T] {
	return func(yield func(T) bool) {
		f := start(feedCap, seqs...)
		defer f.stop() // if the consumer panics
		for v := range f.ch {
			if !yield(v) {
				break
			}
		}
		f.finish()
	}
}

// ChunkedWithin returns a stage that batches by size or by time, whichever
// comes first: it yields a chunk when it holds n elements, or when d has
// passed since the chunk's first element arrived, and yields the last
// partial chunk when the source ends. It never yields an empty chunk, so
// a silent source yields nothing. Every chunk is a fresh slice. T cannot
// be inferred — write parallel.ChunkedWithin[int](100, time.Second). ⚠ Starts
// one goroutine per iteration, joined before it returns, so stopping
// waits for the source's current step. ⚠ Holds up to n elements in the
// chunk and runs the source up to another n ahead (64 at most), plus one. ⚠ Everything upstream runs
// concurrently with the consumer's loop body. Panics if n <= 0 or d <= 0.
func ChunkedWithin[T any](n int, d time.Duration) func(catena.Seq[T]) catena.Seq[[]T] {
	posCheck("ChunkedWithin", "n", n)
	posCheck("ChunkedWithin", "d", d)
	return func(s catena.Seq[T]) catena.Seq[[]T] {
		return func(yield func([]T) bool) {
			f := start(min(n, feedCap), s)
			defer f.stop() // if the consumer panics
			timer := time.NewTimer(d)
			timer.Stop()
			defer timer.Stop()
			var chunk []T
			for {
				var deadline <-chan time.Time // nil, so never ready, while the chunk is empty
				if len(chunk) > 0 {
					deadline = timer.C
				}
				select {
				case v, ok := <-f.ch:
					if !ok {
						f.finish() // a source that panicked loses its partial chunk
						if len(chunk) > 0 {
							yield(chunk)
						}
						return
					}
					if len(chunk) == 0 {
						chunk = make([]T, 0, n) // fresh: never reuse an emitted slice
						timer.Reset(d)          // the window opens at the chunk's first element
					}
					if chunk = append(chunk, v); len(chunk) < n {
						continue
					}
					timer.Stop()
				case <-deadline:
				}
				if !yield(chunk) {
					f.finish()
					return
				}
				chunk = nil
			}
		}
	}
}

// feedCap bounds how far the sources of Merge and ChunkedWithin run ahead
// of the consumer. Handing elements over one at a time, with nothing in
// between, costs a goroutine switch each; a little room lets a fast
// source and a fast consumer each keep running.
const feedCap = 64

// feed is the producer side of the stages that run their sources on
// goroutines: every element of every source arrives on ch, which is
// closed once all of them have ended.
type feed[T any] struct {
	ch      chan T
	done    chan struct{} // closed to make every producer stop
	abort   func()        // closes done, once
	crashes []any         // per source: what to re-raise, if it panicked
	wg      sync.WaitGroup
}

// start runs each source on its own goroutine, sending into a channel of
// capacity buf. A source runs on exactly one goroutine and is never
// called concurrently with itself.
func start[T any](buf int, seqs ...catena.Seq[T]) *feed[T] {
	f := &feed[T]{
		ch:      make(chan T, buf),
		done:    make(chan struct{}),
		crashes: make([]any, len(seqs)),
	}
	f.abort = sync.OnceFunc(func() { close(f.done) })
	var producers sync.WaitGroup
	for i, s := range seqs {
		producers.Go(func() {
			defer func() {
				if f.crashes[i] != nil {
					f.abort() // one source panicking stops the rest
				}
			}()
			protect(&f.crashes[i], func() {
				s.Seq()(func(v T) bool {
					select { // a send that could block must also see done
					case f.ch <- v:
						return true
					case <-f.done:
						return false
					}
				})
			})
		})
	}
	// Closing ch is what publishes crashes to the consumer: it happens
	// after every producer has finished writing its own.
	f.wg.Go(func() {
		producers.Wait()
		close(f.ch)
	})
	return f
}

// stop makes every producer return at its next send and waits for them.
func (f *feed[T]) stop() {
	f.abort()
	f.wg.Wait()
}

// finish stops the producers and then re-raises a source's panic, if one
// panicked — including one that panicked only on being told to stop, as a
// source's own cleanup can. It is how every exit but a consumer's panic
// ends: that one keeps its own panic, and only stops.
func (f *feed[T]) finish() {
	f.stop()
	for _, crash := range f.crashes {
		if crash != nil {
			panic(crash)
		}
	}
}
