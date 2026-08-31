package catena

import "iter"

// Seq2 is a bridge back to Seq, not a peer surface (§2.2 of the spec). The
// operators deliberately absent from it, and what to use instead, are listed
// on the Seq2 type in catena.go.
//
// Bodies use direct source invocation, not range — see the note in seq.go.

// Filter yields the pairs pred admits.
func (s Seq2[K, V]) Filter(pred func(K, V) bool) Seq2[K, V] {
	return func(yield func(K, V) bool) {
		src2(s)(func(k K, v V) bool {
			return !pred(k, v) || yield(k, v)
		})
	}
}

// FilterNot yields the pairs pred rejects.
func (s Seq2[K, V]) FilterNot(pred func(K, V) bool) Seq2[K, V] {
	return func(yield func(K, V) bool) {
		src2(s)(func(k K, v V) bool {
			return pred(k, v) || yield(k, v)
		})
	}
}

// Map yields f applied to each pair.
func (s Seq2[K, V]) Map[K2, V2 any](f func(K, V) (K2, V2)) Seq2[K2, V2] {
	return func(yield func(K2, V2) bool) {
		src2(s)(func(k K, v V) bool {
			return yield(f(k, v))
		})
	}
}

// MapValues yields each pair with its value replaced by f(k, v). f
// receives the key too (Kotlin-consistent).
func (s Seq2[K, V]) MapValues[V2 any](f func(K, V) V2) Seq2[K, V2] {
	return func(yield func(K, V2) bool) {
		src2(s)(func(k K, v V) bool {
			return yield(k, f(k, v))
		})
	}
}

// MapTo collapses each pair into one value — the intended exit back to
// Seq and its full API.
func (s Seq2[K, V]) MapTo[U any](f func(K, V) U) Seq[U] {
	return func(yield func(U) bool) {
		src2(s)(func(k K, v V) bool {
			return yield(f(k, v))
		})
	}
}

// Take yields at most the first n pairs. Panics if n is negative.
func (s Seq2[K, V]) Take(n int) Seq2[K, V] {
	negCheck("Take", n)
	return func(yield func(K, V) bool) {
		if n == 0 {
			return
		}
		left := n
		src2(s)(func(k K, v V) bool {
			if !yield(k, v) {
				return false
			}
			left--
			return left > 0
		})
	}
}

// Drop skips the first n pairs. Panics if n is negative.
func (s Seq2[K, V]) Drop(n int) Seq2[K, V] {
	negCheck("Drop", n)
	return func(yield func(K, V) bool) {
		left := n
		src2(s)(func(k K, v V) bool {
			if left > 0 {
				left--
				return true
			}
			return yield(k, v)
		})
	}
}

// Keys yields the first element of each pair. Calling Keys and Values on
// the same single-pass Seq2 is a double consume — use Unzip.
func (s Seq2[K, V]) Keys() Seq[K] {
	return func(yield func(K) bool) {
		src2(s)(func(k K, _ V) bool {
			return yield(k)
		})
	}
}

// Values yields the second element of each pair.
func (s Seq2[K, V]) Values() Seq[V] {
	return func(yield func(V) bool) {
		src2(s)(func(_ K, v V) bool {
			return yield(v)
		})
	}
}

// Swap yields each pair with its sides exchanged.
func (s Seq2[K, V]) Swap() Seq2[V, K] {
	return func(yield func(V, K) bool) {
		src2(s)(func(k K, v V) bool {
			return yield(v, k)
		})
	}
}

// Fold reduces the pairs into an accumulator, left to right. ⚠ Full drain.
func (s Seq2[K, V]) Fold[A any](init A, f func(A, K, V) A) A {
	acc := init
	src2(s)(func(k K, v V) bool {
		acc = f(acc, k, v)
		return true
	})
	return acc
}

// ForEach calls f on every pair. ⚠ Full drain.
func (s Seq2[K, V]) ForEach(f func(K, V)) {
	src2(s)(func(k K, v V) bool {
		f(k, v)
		return true
	})
}

// Any reports whether pred admits any pair; stops at the first match.
func (s Seq2[K, V]) Any(pred func(K, V) bool) bool {
	found := false
	src2(s)(func(k K, v V) bool {
		found = pred(k, v)
		return !found
	})
	return found
}

// All reports whether pred admits every pair; stops at the first
// counterexample. Vacuously true on empty input.
func (s Seq2[K, V]) All(pred func(K, V) bool) bool {
	ok := true
	src2(s)(func(k K, v V) bool {
		ok = pred(k, v)
		return ok
	})
	return ok
}

// Count returns the number of pairs. ⚠ Full drain.
func (s Seq2[K, V]) Count() int {
	n := 0
	src2(s)(func(K, V) bool {
		n++
		return true
	})
	return n
}

// First returns the first pair.
func (s Seq2[K, V]) First() (K, V, bool) {
	var gk K
	var gv V
	found := false
	src2(s)(func(k K, v V) bool {
		gk, gv, found = k, v, true
		return false
	})
	if !found {
		var zk K
		var zv V
		return zk, zv, false
	}
	return gk, gv, true
}

// Seq2 converts to the stdlib iterator type. Free.
func (s Seq2[K, V]) Seq2() iter.Seq2[K, V] {
	return src2(s)
}

// Pull converts s to a pull-based iterator. THE CALLER MUST CALL stop,
// even if next has returned false, or resources held by s will leak.
func (s Seq2[K, V]) Pull() (next func() (K, V, bool), stop func()) {
	if s == nil {
		return func() (K, V, bool) {
			var zk K
			var zv V
			return zk, zv, false
		}, func() {}
	}
	return iter.Pull2(iter.Seq2[K, V](s))
}
