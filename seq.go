package catena

// Type-preserving intermediates and the two lifecycle guards (Once,
// UntilDone). Operators that introduce a new type parameter live in
// seq_map.go; terminals in seq_terminal.go, seq_fold.go, seq_aggregate.go.
//
// Operators compose by invoking the source sequence directly with a
// wrapper closure rather than with a range statement: `for v := range s`
// inside an operator pays the compiler's rangefunc state machine per
// stage, while `src(s)(func(v T) bool { ... })` is a plain call chain.
// Measured at ~0.3-0.9ns/element/stage — see bench_baseline_test.go.
// The wrapper returns false to stop the source; it must never call yield
// after yield returns false (conformance C6 enforces both).

import (
	"context"
	"slices"
)

// Filter yields the elements for which pred returns true.
func (s Seq[T]) Filter(pred func(T) bool) Seq[T] {
	return func(yield func(T) bool) {
		src(s)(func(v T) bool {
			if !pred(v) {
				return true
			}
			return yield(v)
		})
	}
}

// FilterNot yields the elements for which pred returns false.
func (s Seq[T]) FilterNot(pred func(T) bool) Seq[T] {
	return func(yield func(T) bool) {
		src(s)(func(v T) bool {
			if pred(v) {
				return true
			}
			return yield(v)
		})
	}
}

// FilterIndexed yields the elements for which pred(index, element) returns
// true. The index counts source elements from 0.
func (s Seq[T]) FilterIndexed(pred func(int, T) bool) Seq[T] {
	return func(yield func(T) bool) {
		i := 0
		src(s)(func(v T) bool {
			ok := !pred(i, v) || yield(v)
			i++
			return ok
		})
	}
}

// Take yields at most the first n elements, consuming exactly as many as
// it yields. Panics if n is negative.
func (s Seq[T]) Take(n int) Seq[T] {
	negCheck("Take", n)
	return func(yield func(T) bool) {
		if n == 0 {
			return
		}
		left := n
		src(s)(func(v T) bool {
			if !yield(v) {
				return false
			}
			left--
			return left > 0
		})
	}
}

// TakeWhile yields elements until pred first returns false.
func (s Seq[T]) TakeWhile(pred func(T) bool) Seq[T] {
	return func(yield func(T) bool) {
		src(s)(func(v T) bool {
			return pred(v) && yield(v)
		})
	}
}

// TakeLast yields the final n elements. ⚠ Buffers n elements and fully
// drains the source before emitting — hangs on infinite input. Panics if n
// is negative.
func (s Seq[T]) TakeLast(n int) Seq[T] {
	negCheck("TakeLast", n)
	return func(yield func(T) bool) {
		if n == 0 {
			return
		}
		ring := make([]T, 0, n)
		start := 0
		src(s)(func(v T) bool {
			if len(ring) < n {
				ring = append(ring, v)
			} else {
				ring[start] = v
				start = (start + 1) % n
			}
			return true
		})
		for i := range ring {
			if !yield(ring[(start+i)%len(ring)]) {
				return
			}
		}
	}
}

// Drop skips the first n elements. Panics if n is negative.
func (s Seq[T]) Drop(n int) Seq[T] {
	negCheck("Drop", n)
	return func(yield func(T) bool) {
		left := n
		src(s)(func(v T) bool {
			if left > 0 {
				left--
				return true
			}
			return yield(v)
		})
	}
}

// DropWhile skips elements until pred first returns false, then yields the
// rest.
func (s Seq[T]) DropWhile(pred func(T) bool) Seq[T] {
	return func(yield func(T) bool) {
		dropping := true
		src(s)(func(v T) bool {
			if dropping {
				if pred(v) {
					return true
				}
				dropping = false
			}
			return yield(v)
		})
	}
}

// DropLast yields all but the final n elements, emitting with an
// n-element lag. Panics if n is negative. ⚠ Buffers n elements.
func (s Seq[T]) DropLast(n int) Seq[T] {
	negCheck("DropLast", n)
	return func(yield func(T) bool) {
		if n == 0 {
			src(s)(yield)
			return
		}
		ring := make([]T, n)
		count := 0
		src(s)(func(v T) bool {
			i := count % n
			ok := count < n || yield(ring[i])
			ring[i] = v
			count++
			return ok
		})
	}
}

// Step yields the first element and every nth element after it. Panics if
// n <= 0.
func (s Seq[T]) Step(n int) Seq[T] {
	posCheck("Step", "n", n)
	return func(yield func(T) bool) {
		i := 0
		src(s)(func(v T) bool {
			ok := i%n != 0 || yield(v)
			i++
			return ok
		})
	}
}

// OnEach calls f on every element and passes it through unchanged.
func (s Seq[T]) OnEach(f func(T)) Seq[T] {
	return func(yield func(T) bool) {
		src(s)(func(v T) bool {
			f(v)
			return yield(v)
		})
	}
}

// forward adapts yield into a source callback that records whether the
// consumer stopped — for operators that chain several sources.
func forward[T any](yield func(T) bool, stopped *bool) func(T) bool {
	return func(v T) bool {
		if !yield(v) {
			*stopped = true
			return false
		}
		return true
	}
}

// Concat yields s, then each of the others in order.
func (s Seq[T]) Concat(others ...Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		stopped := false
		fw := forward(yield, &stopped)
		src(s)(fw)
		for _, o := range others {
			if stopped {
				return
			}
			src(o)(fw)
		}
	}
}

// Append yields s, then the given values.
func (s Seq[T]) Append(vals ...T) Seq[T] {
	return func(yield func(T) bool) {
		stopped := false
		src(s)(forward(yield, &stopped))
		if stopped {
			return
		}
		for _, v := range vals {
			if !yield(v) {
				return
			}
		}
	}
}

// Prepend yields the given values, then s.
func (s Seq[T]) Prepend(vals ...T) Seq[T] {
	return func(yield func(T) bool) {
		for _, v := range vals {
			if !yield(v) {
				return
			}
		}
		src(s)(yield)
	}
}

// Intersperse yields sep between consecutive elements.
func (s Seq[T]) Intersperse(sep T) Seq[T] {
	return func(yield func(T) bool) {
		first := true
		src(s)(func(v T) bool {
			if !first && !yield(sep) {
				return false
			}
			first = false
			return yield(v)
		})
	}
}

// IfEmpty yields s, or the given defaults if s yields nothing.
func (s Seq[T]) IfEmpty(defaults ...T) Seq[T] {
	return func(yield func(T) bool) {
		yielded := false
		src(s)(func(v T) bool {
			yielded = true
			return yield(v)
		})
		if !yielded {
			for _, v := range defaults {
				if !yield(v) {
					return
				}
			}
		}
	}
}

// SortedWith yields the elements sorted by cmp, stably. ⚠ Buffers the
// entire input — hangs on infinite input.
func (s Seq[T]) SortedWith(cmp func(a, b T) int) Seq[T] {
	return func(yield func(T) bool) {
		buf := s.Collect()
		slices.SortStableFunc(buf, cmp)
		for _, v := range buf {
			if !yield(v) {
				return
			}
		}
	}
}

// DistinctWith yields elements no earlier element equals under eq. First
// occurrence wins. ⚠ Retains all distinct elements and compares in
// O(n²) — small inputs only.
func (s Seq[T]) DistinctWith(eq func(a, b T) bool) Seq[T] {
	return func(yield func(T) bool) {
		var seen []T
		src(s)(func(v T) bool {
			if slices.ContainsFunc(seen, func(u T) bool { return eq(u, v) }) {
				return true
			}
			seen = append(seen, v)
			return yield(v)
		})
	}
}

// Reversed yields the elements in reverse order. ⚠ Buffers the entire
// input — hangs on infinite input.
func (s Seq[T]) Reversed() Seq[T] {
	return func(yield func(T) bool) {
		buf := s.Collect()
		for i := len(buf) - 1; i >= 0; i-- {
			if !yield(buf[i]) {
				return
			}
		}
	}
}

// Once returns a sequence that panics if iterated more than once — a
// development guard for the single-pass contract, not a synchronization
// mechanism. This is the one operator whose state deliberately lives
// outside the iteration closure.
//
//catena:seq-only
func (s Seq[T]) Once() Seq[T] {
	used := false
	return func(yield func(T) bool) {
		if used {
			panic("catena: Once: sequence consumed more than once")
		}
		used = true
		src(s)(yield)
	}
}

// UntilDone passes elements through until ctx is done, then yields
// (zero, ctx.Err()) and stops.
//
//catena:seq-only
func (s Seq[T]) UntilDone(ctx context.Context) Try[T] {
	return func(yield func(T, error) bool) {
		src(s)(func(v T) bool {
			if err := ctx.Err(); err != nil {
				var zero T
				yield(zero, err)
				return false
			}
			return yield(v, nil)
		})
	}
}
