package catena

// Consumption, search, and predicate terminals, plus the escape hatches to
// the rest of the ecosystem (Seq, Pull, ToChan). Folds and grouping live
// in seq_fold.go; ordered and numeric aggregation in seq_aggregate.go.
//
// Terminals consume the source by direct invocation (src(s)(func ...))
// rather than range statements, for the same reason the operators in
// seq.go do — it skips the compiler's rangefunc state machine.

import (
	"context"
	"iter"
)

// Collect drains the sequence into a slice; nil for empty. ToList is the
// same drain returning a List, which carries the eager operator set.
// ⚠ Full drain.
func (s Seq[T]) Collect() []T {
	var g gatherer[T]
	src(s)(func(v T) bool {
		g.add(v)
		return true
	})
	return g.slice()
}

// ToList drains the sequence into a List; nil for empty. ⚠ Full drain.
func (s Seq[T]) ToList() List[T] {
	return List[T](s.Collect())
}

// Seq converts to the stdlib iterator type. Free.
//
//catena:seq-only
func (s Seq[T]) Seq() iter.Seq[T] {
	return src(s)
}

// Pull converts s to a pull-based iterator. THE CALLER MUST CALL stop,
// even if next has returned false, or resources held by s will leak.
//
//catena:seq-only
func (s Seq[T]) Pull() (next func() (T, bool), stop func()) {
	if s == nil {
		return func() (T, bool) { var zero T; return zero, false }, func() {}
	}
	return iter.Pull(iter.Seq[T](s))
}

// ToChan starts a goroutine that sends every element on the returned
// unbuffered channel. The channel is closed when the sequence ends or ctx
// is done — the consumer must drain or cancel, or the goroutine leaks.
//
//catena:seq-only
func (s Seq[T]) ToChan(ctx context.Context) <-chan T {
	ch := make(chan T)
	if s == nil {
		close(ch)
		return ch
	}
	go func() {
		defer close(ch)
		for v := range iter.Seq[T](s) {
			select {
			case ch <- v:
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch
}

// ForEach calls f on every element. ⚠ Full drain.
func (s Seq[T]) ForEach(f func(T)) {
	src(s)(func(v T) bool {
		f(v)
		return true
	})
}

// ForEachIndexed calls f(index, element) on every element. ⚠ Full drain.
func (s Seq[T]) ForEachIndexed(f func(int, T)) {
	i := 0
	src(s)(func(v T) bool {
		f(i, v)
		i++
		return true
	})
}

// ForEachErr calls f on every element, stopping at and returning the first
// non-nil error; nil if the sequence drains clean.
func (s Seq[T]) ForEachErr(f func(T) error) error {
	var err error
	src(s)(func(v T) bool {
		err = f(v)
		return err == nil
	})
	return err
}

// Drain consumes the sequence for its side effects. ⚠ Full drain.
func (s Seq[T]) Drain() {
	src(s)(func(T) bool { return true })
}

// First returns the first element.
func (s Seq[T]) First() (T, bool) {
	var got T
	found := false
	src(s)(func(v T) bool {
		got, found = v, true
		return false
	})
	if !found {
		var zero T
		return zero, false
	}
	return got, true
}

// Last returns the final element. ⚠ Full drain.
func (s Seq[T]) Last() (T, bool) {
	var last T
	found := false
	src(s)(func(v T) bool {
		last, found = v, true
		return true
	})
	return last, found
}

// Single returns the element iff the sequence has exactly one; it stops
// consuming upon seeing a second.
func (s Seq[T]) Single() (T, bool) {
	var got T
	n := 0
	src(s)(func(v T) bool {
		got = v
		n++
		return n < 2
	})
	if n != 1 {
		var zero T
		return zero, false
	}
	return got, true
}

// ElementAt returns the element at index i; (zero, false) for a negative
// or out-of-range index.
func (s Seq[T]) ElementAt(i int) (T, bool) {
	var got T
	found := false
	if i >= 0 {
		src(s)(func(v T) bool {
			if i == 0 {
				got, found = v, true
				return false
			}
			i--
			return true
		})
	}
	if !found {
		var zero T
		return zero, false
	}
	return got, true
}

// Find returns the first element pred admits.
func (s Seq[T]) Find(pred func(T) bool) (T, bool) {
	var got T
	found := false
	src(s)(func(v T) bool {
		if pred(v) {
			got, found = v, true
			return false
		}
		return true
	})
	if !found {
		var zero T
		return zero, false
	}
	return got, true
}

// FindLast returns the final element pred admits. ⚠ Full drain.
func (s Seq[T]) FindLast(pred func(T) bool) (T, bool) {
	var last T
	found := false
	src(s)(func(v T) bool {
		if pred(v) {
			last, found = v, true
		}
		return true
	})
	return last, found
}

// FindIndex returns the index of the first element pred admits; -1 if
// none.
func (s Seq[T]) FindIndex(pred func(T) bool) int {
	i, at := 0, -1
	src(s)(func(v T) bool {
		if pred(v) {
			at = i
			return false
		}
		i++
		return true
	})
	return at
}

// FindMap returns the first mapped value f reports true for — a fused
// Find + Map.
func (s Seq[T]) FindMap[U any](f func(T) (U, bool)) (U, bool) {
	var got U
	found := false
	src(s)(func(v T) bool {
		if u, ok := f(v); ok {
			got, found = u, true
			return false
		}
		return true
	})
	if !found {
		var zero U
		return zero, false
	}
	return got, true
}

// Any reports whether pred admits any element; stops at the first match.
func (s Seq[T]) Any(pred func(T) bool) bool {
	found := false
	src(s)(func(v T) bool {
		found = pred(v)
		return !found
	})
	return found
}

// All reports whether pred admits every element; stops at the first
// counterexample. Vacuously true on empty input.
func (s Seq[T]) All(pred func(T) bool) bool {
	ok := true
	src(s)(func(v T) bool {
		ok = pred(v)
		return ok
	})
	return ok
}

// None reports whether pred admits no element; stops at the first match.
func (s Seq[T]) None(pred func(T) bool) bool {
	return !s.Any(pred)
}

// Count returns the number of elements. ⚠ Full drain.
func (s Seq[T]) Count() int {
	n := 0
	src(s)(func(T) bool {
		n++
		return true
	})
	return n
}

// CountWhere returns the number of elements pred admits. ⚠ Full drain.
func (s Seq[T]) CountWhere(pred func(T) bool) int {
	n := 0
	src(s)(func(v T) bool {
		if pred(v) {
			n++
		}
		return true
	})
	return n
}

// IsEmpty reports whether the sequence yields nothing. ⚠ It does so by
// consuming one element — on a single-pass source that element is lost.
func (s Seq[T]) IsEmpty() bool {
	empty := true
	src(s)(func(T) bool {
		empty = false
		return false
	})
	return empty
}
