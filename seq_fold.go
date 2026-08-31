package catena

// The fold family and its keyed cousins.

// Fold reduces the sequence into an accumulator, left to right.
//
// Reduce is the same operation using the first element as the initial
// accumulator; FoldBy folds per key in one streaming pass. ⚠ Full drain.
func (s Seq[T]) Fold[A any](init A, f func(A, T) A) A {
	acc := init
	src(s)(func(v T) bool {
		acc = f(acc, v)
		return true
	})
	return acc
}

// FoldIndexed is Fold with the element index. ⚠ Full drain.
func (s Seq[T]) FoldIndexed[A any](init A, f func(int, A, T) A) A {
	acc := init
	i := 0
	src(s)(func(v T) bool {
		acc = f(i, acc, v)
		i++
		return true
	})
	return acc
}

// FoldWhile folds until f reports false; the accumulator from the stopping
// call is included in the result.
func (s Seq[T]) FoldWhile[A any](init A, f func(A, T) (A, bool)) A {
	acc := init
	src(s)(func(v T) bool {
		var cont bool
		acc, cont = f(acc, v)
		return cont
	})
	return acc
}

// FoldErr folds until f fails, returning the accumulator so far and the
// first error.
func (s Seq[T]) FoldErr[A any](init A, f func(A, T) (A, error)) (A, error) {
	acc := init
	var err error
	src(s)(func(v T) bool {
		var next A
		next, err = f(acc, v)
		if err != nil {
			return false
		}
		acc = next
		return true
	})
	return acc, err
}

// FoldBy folds each element into a per-key accumulator: streaming grouped
// aggregation with no intermediate per-key slices. init is called once per
// distinct key. Memory is bounded by the number of distinct keys, not
// elements. ⚠ Full drain; map iteration order is undefined.
func (s Seq[T]) FoldBy[K comparable, A any](
	key func(T) K, init func(K) A, f func(A, T) A,
) map[K]A {
	out := make(map[K]A)
	src(s)(func(v T) bool {
		k := key(v)
		acc, ok := out[k]
		if !ok {
			acc = init(k)
		}
		out[k] = f(acc, v)
		return true
	})
	return out
}

// Reduce folds the sequence using its first element as the initial
// accumulator; (zero, false) on empty input. ⚠ Full drain.
func (s Seq[T]) Reduce(f func(T, T) T) (T, bool) {
	var acc T
	have := false
	src(s)(func(v T) bool {
		if !have {
			acc, have = v, true
		} else {
			acc = f(acc, v)
		}
		return true
	})
	return acc, have
}

// GroupBy collects elements into per-key buckets, each in encounter order.
// ⚠ Full drain; retains every element; map iteration order is undefined.
func (s Seq[T]) GroupBy[K comparable](sel func(T) K) map[K][]T {
	out := make(map[K][]T)
	src(s)(func(v T) bool {
		k := sel(v)
		out[k] = append(out[k], v)
		return true
	})
	return out
}

// IndexBy maps each key to its element; on duplicate keys the last element
// wins. ⚠ Full drain; map iteration order is undefined.
func (s Seq[T]) IndexBy[K comparable](sel func(T) K) map[K]T {
	out := make(map[K]T)
	src(s)(func(v T) bool {
		out[sel(v)] = v
		return true
	})
	return out
}

// TallyBy counts elements per key. ⚠ Full drain; map iteration order is
// undefined.
func (s Seq[T]) TallyBy[K comparable](sel func(T) K) map[K]int {
	out := make(map[K]int)
	src(s)(func(v T) bool {
		out[sel(v)]++
		return true
	})
	return out
}

// Associate builds a map from f's key/value pairs; on duplicate keys the
// last pair wins. ⚠ Full drain; map iteration order is undefined.
func (s Seq[T]) Associate[K comparable, V any](f func(T) (K, V)) map[K]V {
	out := make(map[K]V)
	src(s)(func(v T) bool {
		k, val := f(v)
		out[k] = val
		return true
	})
	return out
}

// Partition splits elements by pred, preserving encounter order on both
// sides; nil slices for empty sides. ⚠ Full drain.
func (s Seq[T]) Partition(pred func(T) bool) (yes, no []T) {
	var gy, gn gatherer[T]
	src(s)(func(v T) bool {
		if pred(v) {
			gy.add(v)
		} else {
			gn.add(v)
		}
		return true
	})
	return gy.slice(), gn.slice()
}
