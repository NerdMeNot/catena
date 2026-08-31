package catena

import (
	"context"
)

// Of returns a re-iterable Seq over the given values.
func Of[T any](vals ...T) Seq[T] {
	return FromSlice(vals)
}

// From adapts any push-function sequence — iter.Seq, catena.Seq, or a
// third-party alias — with no conversion at the call site. Re-iterability
// depends on the source.
func From[T any](seq func(func(T) bool)) Seq[T] {
	return Seq[T](seq)
}

// From2 adapts any push-function pair sequence. Re-iterable iff the
// underlying source is.
func From2[K, V any](seq func(func(K, V) bool)) Seq2[K, V] {
	return Seq2[K, V](seq)
}

// FromErrs adapts any push-function fallible sequence. Re-iterable iff
// the underlying source is.
func FromErrs[T any](seq func(func(T, error) bool)) Try[T] {
	return Try[T](seq)
}

// FromSlice returns a re-iterable Seq over s. The slice is not copied;
// mutations to it are visible to later iterations.
func FromSlice[T any](s []T) Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range s {
			if !yield(v) {
				return
			}
		}
	}
}

// FromMap returns a re-iterable Seq2 over m, in undefined (map) order.
func FromMap[K comparable, V any](m map[K]V) Seq2[K, V] {
	return func(yield func(K, V) bool) {
		for k, v := range m {
			if !yield(k, v) {
				return
			}
		}
	}
}

// FromChan yields values received from ch until ch is closed or ctx is
// done. Single-use. No goroutine is started; a sequence that is never
// consumed never receives.
func FromChan[T any](ctx context.Context, ch <-chan T) Seq[T] {
	return func(yield func(T) bool) {
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-ch:
				if !ok {
					return
				}
				if !yield(v) {
					return
				}
			}
		}
	}
}

// Empty returns the empty Seq. Re-iterable.
func Empty[T any]() Seq[T] {
	return func(func(T) bool) {}
}

// Empty2 returns the empty Seq2. Re-iterable.
func Empty2[K, V any]() Seq2[K, V] {
	return func(func(K, V) bool) {}
}

// EmptyTry returns the empty Try. Re-iterable.
func EmptyTry[T any]() Try[T] {
	return func(func(T, error) bool) {}
}

// Once1 returns a re-iterable Seq of exactly one value. (Once, without the
// suffix, is the single-use guard method on Seq.)
func Once1[T any](v T) Seq[T] {
	return func(yield func(T) bool) {
		yield(v)
	}
}

// Repeat yields v forever. Re-iterable. Infinite: pair with Take or a
// conditional terminal.
func Repeat[T any](v T) Seq[T] {
	return func(yield func(T) bool) {
		for yield(v) {
		}
	}
}

// RepeatN yields v exactly n times. Re-iterable. Panics if n is negative.
func RepeatN[T any](v T, n int) Seq[T] {
	negCheck("RepeatN", n)
	return func(yield func(T) bool) {
		for range n {
			if !yield(v) {
				return
			}
		}
	}
}

// Generate yields seed, then next(seed), then next(next(seed)), forever.
// Infinite. Re-iterable iff next is pure.
func Generate[T any](seed T, next func(T) T) Seq[T] {
	return func(yield func(T) bool) {
		for v := seed; yield(v); v = next(v) {
		}
	}
}

// GenerateWhile yields seed unconditionally, then successive next values
// until next reports false. Re-iterable iff next is pure.
func GenerateWhile[T any](seed T, next func(T) (T, bool)) Seq[T] {
	return func(yield func(T) bool) {
		v := seed
		for {
			if !yield(v) {
				return
			}
			var ok bool
			if v, ok = next(v); !ok {
				return
			}
		}
	}
}

// Range yields start, start+step, ... while the value is before stop
// (exclusive). Re-iterable. step == 0 panics at construction; a sign mismatch between
// step and the start→stop direction yields an empty sequence. Termination
// is overflow-guarded: a step past the type's edge stops rather than
// wrapping. Unsigned types cannot step downward.
func Range[I Integer](start, stop, step I) Seq[I] {
	var zero I
	if step == zero {
		panic("catena: Range: step must be non-zero")
	}
	return func(yield func(I) bool) {
		if step > zero {
			for v := start; v < stop; {
				if !yield(v) {
					return
				}
				next := v + step
				if next < v { // overflow wrap
					return
				}
				v = next
			}
		} else {
			for v := start; v > stop; {
				if !yield(v) {
					return
				}
				next := v + step
				if next > v { // underflow wrap
					return
				}
				v = next
			}
		}
	}
}

// Cycle yields s over and over, forever. The first pass is buffered and
// replayed (⚠ unbounded memory in len(s)), so Cycle is re-iterable iff
// that first pass of s is. An empty s yields an empty Cycle — it
// terminates rather than spinning.
func Cycle[T any](s Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		var g gatherer[T]
		stopped := false
		src(s)(func(v T) bool {
			g.add(v)
			if !yield(v) {
				stopped = true
				return false
			}
			return true
		})
		buf := g.slice()
		if stopped || len(buf) == 0 {
			return
		}
		for {
			for _, v := range buf {
				if !yield(v) {
					return
				}
			}
		}
	}
}
