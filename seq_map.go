package catena

// Intermediates that carry the chain into a new type: mapping, flattening,
// pairing, keyed selection, keyed sorting, and the relational join. These
// are the operators Go could not express as methods before 1.27 generic
// methods. Composition is by direct source invocation — see seq.go.

import (
	"cmp"
	"iter"
	"slices"
)

// Map yields f applied to each element.
func (s Seq[T]) Map[U any](f func(T) U) Seq[U] {
	return func(yield func(U) bool) {
		src(s)(func(v T) bool {
			return yield(f(v))
		})
	}
}

// MapIndexed yields f(index, element), counting from 0.
func (s Seq[T]) MapIndexed[U any](f func(int, T) U) Seq[U] {
	return func(yield func(U) bool) {
		i := 0
		src(s)(func(v T) bool {
			ok := yield(f(i, v))
			i++
			return ok
		})
	}
}

// FilterMap yields the mapped value for each element f reports true for —
// a fused Map + Filter.
func (s Seq[T]) FilterMap[U any](f func(T) (U, bool)) Seq[U] {
	return func(yield func(U) bool) {
		src(s)(func(v T) bool {
			u, ok := f(v)
			return !ok || yield(u)
		})
	}
}

// FlatMap yields all elements of f(v) for each element v, in order.
func (s Seq[T]) FlatMap[U any](f func(T) Seq[U]) Seq[U] {
	return func(yield func(U) bool) {
		stopped := false
		fw := forward(yield, &stopped)
		src(s)(func(v T) bool {
			src(f(v))(fw)
			return !stopped
		})
	}
}

// FlatMapSlice yields all elements of the slice f(v) for each element v.
func (s Seq[T]) FlatMapSlice[U any](f func(T) []U) Seq[U] {
	return func(yield func(U) bool) {
		src(s)(func(v T) bool {
			for _, u := range f(v) {
				if !yield(u) {
					return false
				}
			}
			return true
		})
	}
}

// Scan yields the running accumulator: f(init, e0), f(that, e1), ... The
// initial value itself is not yielded.
func (s Seq[T]) Scan[A any](init A, f func(A, T) A) Seq[A] {
	return func(yield func(A) bool) {
		acc := init
		src(s)(func(v T) bool {
			acc = f(acc, v)
			return yield(acc)
		})
	}
}

// WithIndex pairs each element with its index, counting from 0.
func (s Seq[T]) WithIndex() Seq2[int, T] {
	return func(yield func(int, T) bool) {
		i := 0
		src(s)(func(v T) bool {
			ok := yield(i, v)
			i++
			return ok
		})
	}
}

// ZipWithNext yields each adjacent pair (element, next element). Empty and
// single-element input yield nothing.
func (s Seq[T]) ZipWithNext() Seq2[T, T] {
	return func(yield func(T, T) bool) {
		var prev T
		have := false
		src(s)(func(v T) bool {
			ok := !have || yield(prev, v)
			prev, have = v, true
			return ok
		})
	}
}

// Zip pairs elements of s with elements of other, stopping at the shorter
// side. The receiver drives; other is consumed through iter.Pull (its
// cleanup always runs). other is pulled once per emitted pair; the
// receiver is consumed one element past the pair count when other is
// shorter.
//
//catena:seq-only
func (s Seq[T]) Zip[U any](other Seq[U]) Seq2[T, U] {
	return func(yield func(T, U) bool) {
		next, stop := iter.Pull(src(other))
		defer stop()
		src(s)(func(v T) bool {
			u, ok := next()
			return ok && yield(v, u)
		})
	}
}

// MapErr yields f applied to each element as a Try; a failed call yields
// (zero, err).
func (s Seq[T]) MapErr[U any](f func(T) (U, error)) Try[U] {
	return func(yield func(U, error) bool) {
		src(s)(func(v T) bool {
			u, err := f(v)
			if err != nil {
				var zero U
				u = zero
			}
			return yield(u, err)
		})
	}
}

// FilterErr yields elements pred admits, as a Try; a failed pred call
// yields (zero, err).
func (s Seq[T]) FilterErr(pred func(T) (bool, error)) Try[T] {
	return func(yield func(T, error) bool) {
		src(s)(func(v T) bool {
			ok, err := pred(v)
			if err != nil {
				var zero T
				return yield(zero, err)
			}
			return !ok || yield(v, nil)
		})
	}
}

// DistinctBy yields elements whose key has not been seen before; the first
// occurrence wins. catena.Distinct(s) is the no-selector form, for a
// comparable T. On key-sorted input DedupeBy is the O(1)-memory equivalent.
// ⚠ Retains one key per distinct value — unbounded.
func (s Seq[T]) DistinctBy[K comparable](sel func(T) K) Seq[T] {
	return func(yield func(T) bool) {
		seen := make(map[K]struct{})
		src(s)(func(v T) bool {
			k := sel(v)
			if _, dup := seen[k]; dup {
				return true
			}
			seen[k] = struct{}{}
			return yield(v)
		})
	}
}

// DedupeBy yields elements whose key differs from the previous element's
// key — consecutive duplicates only, O(1) memory. On key-sorted input it
// equals DistinctBy at a fraction of the cost.
func (s Seq[T]) DedupeBy[K comparable](sel func(T) K) Seq[T] {
	return func(yield func(T) bool) {
		var prev K
		have := false
		src(s)(func(v T) bool {
			k := sel(v)
			if have && k == prev {
				return true
			}
			prev, have = k, true
			return yield(v)
		})
	}
}

// sortedByKey is the shared decorate-sort-undecorate body behind SortedBy
// and SortedByDesc: sel is called exactly once per element, not once per
// comparison, at the price of one decorated buffer (the input is fully
// buffered either way).
func sortedByKey[T any, K cmp.Ordered](s Seq[T], sel func(T) K, desc bool) Seq[T] {
	return func(yield func(T) bool) {
		type entry struct {
			v T
			k K
		}
		var g gatherer[entry]
		src(s)(func(v T) bool {
			g.add(entry{v, sel(v)})
			return true
		})
		buf := g.slice()
		slices.SortStableFunc(buf, func(a, b entry) int {
			if desc {
				return cmp.Compare(b.k, a.k)
			}
			return cmp.Compare(a.k, b.k)
		})
		for _, e := range buf {
			if !yield(e.v) {
				return
			}
		}
	}
}

// SortedBy yields the elements sorted ascending by key, stably. sel is
// called exactly once per element (decorate-sort-undecorate).
// catena.Sorted(s) is the no-selector form, for an ordered T. ⚠ Buffers
// the entire input — hangs on infinite input.
func (s Seq[T]) SortedBy[K cmp.Ordered](sel func(T) K) Seq[T] {
	return sortedByKey(s, sel, false)
}

// SortedByDesc yields the elements sorted descending by key, stably. sel
// is called exactly once per element. ⚠ Buffers the entire input — hangs
// on infinite input.
func (s Seq[T]) SortedByDesc[K cmp.Ordered](sel func(T) K) Seq[T] {
	return sortedByKey(s, sel, true)
}

// JoinBy is a relational inner join: it pairs each element of s with every
// element of other sharing the same key and yields combine for each pair.
// It is unrelated to Join and JoinToString, which concatenate strings.
// Unmatched elements on either side are dropped; duplicate keys produce
// the cross product per key. Output order is left encounter order, then
// right encounter order within a key. ⚠ Buffers all of other before the
// first emission.
func (s Seq[T]) JoinBy[U any, K comparable, R any](
	other Seq[U],
	leftKey func(T) K, rightKey func(U) K,
	combine func(T, U) R,
) Seq[R] {
	return func(yield func(R) bool) {
		right := make(map[K][]U)
		src(other)(func(u U) bool {
			k := rightKey(u)
			right[k] = append(right[k], u)
			return true
		})
		src(s)(func(v T) bool {
			for _, u := range right[leftKey(v)] {
				if !yield(combine(v, u)) {
					return false
				}
			}
			return true
		})
	}
}
