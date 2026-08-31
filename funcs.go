package catena

import (
	"cmp"
	"iter"
	"strings"
)

// Bodies use direct source invocation, not range — see the note in seq.go.

// Distinct yields elements not seen before; the first occurrence wins.
//
// Distinct is a package function, not a method, and so are the others in this
// file: they constrain the element type (comparable, cmp.Ordered, Numeric),
// and a method on Seq[T any] may require nothing of T — or,
// for the Flatten family, they constrain the receiver's shape. Every
// comparable-constrained function panics at runtime if T is an interface type
// holding a non-comparable value. The chain continues normally after them:
// catena.Distinct(s).Filter(f).
//
// On input already sorted by the compared value, Dedupe is the O(1)-memory
// equivalent.
// ⚠ Retains one entry per distinct value — unbounded.
func Distinct[T comparable](s Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		seen := make(map[T]struct{})
		src(s)(func(v T) bool {
			if _, dup := seen[v]; dup {
				return true
			}
			seen[v] = struct{}{}
			return yield(v)
		})
	}
}

// Dedupe yields elements that differ from their predecessor — consecutive
// duplicates only, O(1) memory.
func Dedupe[T comparable](s Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		var prev T
		have := false
		src(s)(func(v T) bool {
			if have && v == prev {
				return true
			}
			prev, have = v, true
			return yield(v)
		})
	}
}

// Sorted yields the elements in ascending order, stably. NaN sorts first.
// ⚠ Buffers the entire input.
func Sorted[T cmp.Ordered](s Seq[T]) Seq[T] {
	return s.SortedWith(cmp.Compare[T])
}

// SortedDesc yields the elements in descending order, stably. ⚠ Buffers
// the entire input.
func SortedDesc[T cmp.Ordered](s Seq[T]) Seq[T] {
	return s.SortedWith(func(a, b T) int { return cmp.Compare(b, a) })
}

// Sum adds the elements; integer overflow wraps like +. Empty input sums
// to 0. ⚠ Full drain.
func Sum[T Numeric](s Seq[T]) T {
	var sum T
	src(s)(func(v T) bool {
		sum += v
		return true
	})
	return sum
}

// Product multiplies the elements. Empty input yields 1, the
// multiplicative identity. ⚠ Full drain.
func Product[T Numeric](s Seq[T]) T {
	prod := T(1)
	src(s)(func(v T) bool {
		prod *= v
		return true
	})
	return prod
}

// Average returns the mean, accumulating in float64; (0, false) on empty
// input. ⚠ Full drain.
func Average[T Numeric](s Seq[T]) (float64, bool) {
	var sum float64
	n := 0
	src(s)(func(v T) bool {
		sum += float64(v)
		n++
		return true
	})
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// Max returns the largest element; NaN orders below everything
// (cmp.Compare). ⚠ Full drain.
func Max[T cmp.Ordered](s Seq[T]) (T, bool) {
	var best T
	found := false
	src(s)(func(v T) bool {
		if !found || cmp.Compare(v, best) > 0 {
			best, found = v, true
		}
		return true
	})
	return best, found
}

// Min returns the smallest element; NaN orders below everything, so a NaN
// in the input is the minimum. ⚠ Full drain.
func Min[T cmp.Ordered](s Seq[T]) (T, bool) {
	var best T
	found := false
	src(s)(func(v T) bool {
		if !found || cmp.Compare(v, best) < 0 {
			best, found = v, true
		}
		return true
	})
	return best, found
}

// MinMax returns the smallest and largest elements in one pass. ⚠ Full
// drain.
func MinMax[T cmp.Ordered](s Seq[T]) (min, max T, ok bool) {
	src(s)(func(v T) bool {
		if !ok {
			min, max, ok = v, v, true
			return true
		}
		if cmp.Compare(v, min) < 0 {
			min = v
		}
		if cmp.Compare(v, max) > 0 {
			max = v
		}
		return true
	})
	return min, max, ok
}

// TopN returns the n largest elements, sorted descending; equal elements
// retain encounter order. Memory is O(n) — the streaming alternative to
// SortedDesc(s).Take(n). ⚠ Full drain. Panics if n is negative.
func TopN[T cmp.Ordered](s Seq[T], n int) []T {
	return topNBy("TopN", s, n, Self[T], true)
}

// BottomN returns the n smallest elements, sorted ascending; equal elements
// retain encounter order. Memory is O(n) — the streaming alternative to
// Sorted(s).Take(n). ⚠ Full drain. Panics if n is negative.
func BottomN[T cmp.Ordered](s Seq[T], n int) []T {
	return topNBy("BottomN", s, n, Self[T], false)
}

// Contains reports whether v occurs in s; stops at the first match.
func Contains[T comparable](s Seq[T], v T) bool {
	found := false
	src(s)(func(u T) bool {
		found = u == v
		return !found
	})
	return found
}

// IndexOf returns the index of the first occurrence of v; -1 if none.
func IndexOf[T comparable](s Seq[T], v T) int {
	i, at := 0, -1
	src(s)(func(u T) bool {
		if u == v {
			at = i
			return false
		}
		i++
		return true
	})
	return at
}

// NonZero yields the elements that are not the zero value of T.
func NonZero[T comparable](s Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		var zero T
		src(s)(func(v T) bool {
			if v == zero {
				return true
			}
			return yield(v)
		})
	}
}

// ToKeySet drains the sequence into a membership map, whose keys are the
// distinct elements. Named for what it returns rather than for the set it
// stands in for: ToSet is reserved for a real set type, should Go ever
// grow one. ⚠ Full drain; map iteration order is undefined.
func ToKeySet[T comparable](s Seq[T]) map[T]struct{} {
	out := make(map[T]struct{})
	src(s)(func(v T) bool {
		out[v] = struct{}{}
		return true
	})
	return out
}

// Tally counts occurrences per value. ⚠ Full drain; map iteration order is
// undefined.
func Tally[T comparable](s Seq[T]) map[T]int {
	out := make(map[T]int)
	src(s)(func(v T) bool {
		out[v]++
		return true
	})
	return out
}

// AssociateWith maps each element to f(element); on duplicate elements the
// last value wins. ⚠ Full drain; map iteration order is undefined.
func AssociateWith[T comparable, V any](s Seq[T], f func(T) V) map[T]V {
	out := make(map[T]V)
	src(s)(func(v T) bool {
		out[v] = f(v)
		return true
	})
	return out
}

// Equal reports whether a and b yield the same elements in the same order.
// Consumes both sequences up to and including the first difference — fully
// when they are equal. b is consumed through iter.Pull (its cleanup always
// runs).
func Equal[T comparable](a, b Seq[T]) bool {
	next, stop := iter.Pull(src(b))
	defer stop()
	eq := true
	src(a)(func(v T) bool {
		u, ok := next()
		eq = ok && u == v
		return eq
	})
	if !eq {
		return false
	}
	_, more := next()
	return !more
}

// Union yields the distinct elements of a, then the distinct elements of b
// not in a — set semantics, encounter order. ⚠ Retains one entry per
// distinct value — unbounded.
func Union[T comparable](a, b Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		seen := make(map[T]struct{})
		stopped := false
		emit := func(v T) bool {
			if _, dup := seen[v]; dup {
				return true
			}
			seen[v] = struct{}{}
			if !yield(v) {
				stopped = true
				return false
			}
			return true
		}
		src(a)(emit)
		if !stopped {
			src(b)(emit)
		}
	}
}

// Intersect yields the distinct elements of a that occur in b, in a's
// encounter order. ⚠ Buffers all of b before a is consumed, plus a seen-
// set of a's distinct values.
func Intersect[T comparable](a, b Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		inB := ToKeySet(b)
		seen := make(map[T]struct{})
		src(a)(func(v T) bool {
			if _, ok := inB[v]; !ok {
				return true
			}
			if _, dup := seen[v]; dup {
				return true
			}
			seen[v] = struct{}{}
			return yield(v)
		})
	}
}

// Except yields the distinct elements of a that do not occur in b, in a's
// encounter order. ⚠ Buffers all of b before a is consumed, plus a seen-
// set of a's distinct values.
func Except[T comparable](a, b Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		inB := ToKeySet(b)
		seen := make(map[T]struct{})
		src(a)(func(v T) bool {
			if _, ok := inB[v]; ok {
				return true
			}
			if _, dup := seen[v]; dup {
				return true
			}
			seen[v] = struct{}{}
			return yield(v)
		})
	}
}

// Flatten yields every element of every inner sequence, in order.
func Flatten[T any](s Seq[Seq[T]]) Seq[T] {
	return s.FlatMap(Self[Seq[T]])
}

// FlattenSlices yields every element of every slice, in order.
func FlattenSlices[T any](s Seq[[]T]) Seq[T] {
	return s.FlatMapSlice(Self[[]T])
}

// Chain yields each sequence's elements in order. It is the package-function
// form of Seq.Concat, for when you hold a slice of sequences and no receiver.
func Chain[T any](seqs ...Seq[T]) Seq[T] {
	return func(yield func(T) bool) {
		stopped := false
		fw := forward(yield, &stopped)
		for _, s := range seqs {
			src(s)(fw)
			if stopped {
				return
			}
		}
	}
}

// Unzip drains a pair sequence into its two sides; nil slices for empty
// input. ⚠ Full drain; buffers both sides.
func Unzip[K, V any](s Seq2[K, V]) ([]K, []V) {
	var ks []K
	var vs []V
	src2(s)(func(k K, v V) bool {
		ks = append(ks, k)
		vs = append(vs, v)
		return true
	})
	return ks, vs
}

// CollectMap drains a pair sequence into a map; on duplicate keys the last
// value wins. ⚠ Full drain; map iteration order is undefined.
func CollectMap[K comparable, V any](s Seq2[K, V]) map[K]V {
	out := make(map[K]V)
	src2(s)(func(k K, v V) bool {
		out[k] = v
		return true
	})
	return out
}

// Join concatenates a string sequence with sep between elements. ⚠ Full
// drain.
func Join(s Seq[string], sep string) string {
	var b strings.Builder
	first := true
	src(s)(func(v string) bool {
		if !first {
			b.WriteString(sep)
		}
		first = false
		b.WriteString(v)
		return true
	})
	return b.String()
}
