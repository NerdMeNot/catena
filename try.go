package catena

import (
	"context"
	"iter"
)

// Try operators follow five uniform rules, R1–R5, stated once on the Try
// type in catena.go and referred to by number throughout this file.
//
// Bodies use direct source invocation, not range — see the note in seq.go.

// Map yields f applied to each successful element; errored elements pass
// through.
func (t Try[T]) Map[U any](f func(T) U) Try[U] {
	return func(yield func(U, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			var u U
			if err == nil {
				u = f(v)
			}
			return yield(u, err)
		})
	}
}

// MapErr yields f applied to each successful element; a failed call yields
// (zero, err); errored elements pass through.
func (t Try[T]) MapErr[U any](f func(T) (U, error)) Try[U] {
	return func(yield func(U, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			var u U
			if err == nil {
				u, err = f(v)
				if err != nil {
					var zero U
					u = zero
				}
			}
			return yield(u, err)
		})
	}
}

// FlatMap yields every element of f(v) for each successful element v, in
// order; an errored input element passes through un-mapped.
func (t Try[T]) FlatMap[U any](f func(T) Try[U]) Try[U] {
	return func(yield func(U, error) bool) {
		stopped := false
		srcTry(t)(func(v T, err error) bool {
			if err != nil {
				var zero U
				return yield(zero, err)
			}
			srcTry(f(v))(func(u U, uerr error) bool {
				if !yield(u, uerr) {
					stopped = true
					return false
				}
				return true
			})
			return !stopped
		})
	}
}

// Filter yields the successful elements pred admits; errored elements pass
// through unexamined.
func (t Try[T]) Filter(pred func(T) bool) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err == nil && !pred(v) {
				return true
			}
			return yield(v, err)
		})
	}
}

// FilterErr yields the successful elements pred admits; a failed pred call
// yields (zero, err); errored elements pass through unexamined.
func (t Try[T]) FilterErr(pred func(T) (bool, error)) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err == nil {
				ok, perr := pred(v)
				if perr != nil {
					var zero T
					return yield(zero, perr)
				}
				if !ok {
					return true
				}
			}
			return yield(v, err)
		})
	}
}

// Take yields at most the first n elements, errored or not (R2). Panics if
// n is negative.
func (t Try[T]) Take(n int) Try[T] {
	negCheck("Take", n)
	return func(yield func(T, error) bool) {
		if n == 0 {
			return
		}
		left := n
		srcTry(t)(func(v T, err error) bool {
			if !yield(v, err) {
				return false
			}
			left--
			return left > 0
		})
	}
}

// TakeWhile yields elements until pred rejects a successful element;
// errored elements pass through and do not terminate (R3).
func (t Try[T]) TakeWhile(pred func(T) bool) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err == nil && !pred(v) {
				return false
			}
			return yield(v, err)
		})
	}
}

// Drop skips the first n elements, errored or not (R2). Panics if n is
// negative.
func (t Try[T]) Drop(n int) Try[T] {
	negCheck("Drop", n)
	return func(yield func(T, error) bool) {
		left := n
		srcTry(t)(func(v T, err error) bool {
			if left > 0 {
				left--
				return true
			}
			return yield(v, err)
		})
	}
}

// OnEach calls f on every successful element and passes everything
// through.
func (t Try[T]) OnEach(f func(T)) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err == nil {
				f(v)
			}
			return yield(v, err)
		})
	}
}

// OnError calls f on every error and passes everything through — a
// logging hook.
func (t Try[T]) OnError(f func(error)) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err != nil {
				f(err)
			}
			return yield(v, err)
		})
	}
}

// Recover offers each error to f: reporting true replaces the element with
// (v, nil); reporting false passes the error through unchanged.
func (t Try[T]) Recover(f func(error) (T, bool)) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err != nil {
				if r, ok := f(err); ok {
					v, err = r, nil
				}
			}
			return yield(v, err)
		})
	}
}

// WrapErr replaces each error with f(err) — the place to add positional
// context. If f returns nil (a caller bug), the original error is kept: an
// error is never converted into a zero-value success.
func (t Try[T]) WrapErr(f func(error) error) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err != nil {
				if w := f(err); w != nil {
					err = w
				}
			}
			return yield(v, err)
		})
	}
}

// UntilDone passes elements through until ctx is done, then yields
// (zero, ctx.Err()) and stops.
func (t Try[T]) UntilDone(ctx context.Context) Try[T] {
	return func(yield func(T, error) bool) {
		srcTry(t)(func(v T, err error) bool {
			if cerr := ctx.Err(); cerr != nil {
				var zero T
				yield(zero, cerr)
				return false
			}
			return yield(v, err)
		})
	}
}

// Collect gathers successful elements until the first error, returning the
// partial slice and that error (R5); (all elements, nil) on a clean drain.
// Nil slice for empty.
func (t Try[T]) Collect() ([]T, error) {
	var out []T
	var ferr error
	srcTry(t)(func(v T, err error) bool {
		if err != nil {
			ferr = err
			return false
		}
		out = append(out, v)
		return true
	})
	return out, ferr
}

// CollectAll drains everything, gathering all successes and all errors.
// Positional correspondence between the two slices is lost. Nil slices
// when empty. ⚠ Full drain.
func (t Try[T]) CollectAll() ([]T, []error) {
	var out []T
	var errs []error
	srcTry(t)(func(v T, err error) bool {
		if err != nil {
			errs = append(errs, err)
		} else {
			out = append(out, v)
		}
		return true
	})
	return out, errs
}

// Fold reduces successful elements until the first error, returning the
// accumulator so far and that error (R5).
func (t Try[T]) Fold[A any](init A, f func(A, T) A) (A, error) {
	acc := init
	var ferr error
	srcTry(t)(func(v T, err error) bool {
		if err != nil {
			ferr = err
			return false
		}
		acc = f(acc, v)
		return true
	})
	return acc, ferr
}

// ForEach calls f on each successful element, stopping at and returning
// the first of an element error or a non-nil f return (R5).
func (t Try[T]) ForEach(f func(T) error) error {
	var ferr error
	srcTry(t)(func(v T, err error) bool {
		if err != nil {
			ferr = err
			return false
		}
		ferr = f(v)
		return ferr == nil
	})
	return ferr
}

// Ignore yields the successful elements, dropping errored ones.
func (t Try[T]) Ignore() Seq[T] {
	return func(yield func(T) bool) {
		srcTry(t)(func(v T, err error) bool {
			return err != nil || yield(v)
		})
	}
}

// Errs yields the errors, dropping successful elements — the dual of
// Ignore. Ignore and Errs on the same single-pass Try is a double
// consume; use CollectAll.
func (t Try[T]) Errs() Seq[error] {
	return func(yield func(error) bool) {
		srcTry(t)(func(_ T, err error) bool {
			return err == nil || yield(err)
		})
	}
}

// Must yields the successful elements and panics with the error value on
// the first error — recover() receives the error itself.
func (t Try[T]) Must() Seq[T] {
	return func(yield func(T) bool) {
		srcTry(t)(func(v T, err error) bool {
			if err != nil {
				panic(err)
			}
			return yield(v)
		})
	}
}

// Err consumes until the first error and returns it; nil on a clean drain
// (R5).
func (t Try[T]) Err() error {
	var ferr error
	srcTry(t)(func(_ T, err error) bool {
		ferr = err
		return err == nil
	})
	return ferr
}

// Count counts successful elements up to the first error, which is
// returned alongside the count so far (R5).
func (t Try[T]) Count() (int, error) {
	n := 0
	var ferr error
	srcTry(t)(func(_ T, err error) bool {
		if err != nil {
			ferr = err
			return false
		}
		n++
		return true
	})
	return n, ferr
}

// Pull converts t to a pull-based iterator. THE CALLER MUST CALL stop,
// even if next has returned false, or resources held by t will leak.
func (t Try[T]) Pull() (next func() (T, error, bool), stop func()) {
	if t == nil {
		return func() (T, error, bool) {
			var zero T
			return zero, nil, false
		}, func() {}
	}
	return iter.Pull2(iter.Seq2[T, error](t))
}

// Seq2 converts to the stdlib iterator type. Free.
func (t Try[T]) Seq2() iter.Seq2[T, error] {
	return srcTry(t)
}
