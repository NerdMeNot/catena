package catena

// Ordered and numeric aggregation. Everything here follows the family
// rules stated once in the package doc: cmp.Compare ordering (NaN sorts
// below everything), first-wins ties, (zero, false) on empty input,
// Product's empty result is 1.

import (
	"cmp"
	"slices"
	"strings"
)

// MaxBy returns the element with the largest key; the earliest maximal
// element wins ties. NaN keys order below everything (cmp.Compare).
// ⚠ Full drain.
func (s Seq[T]) MaxBy[K cmp.Ordered](sel func(T) K) (T, bool) {
	var best T
	var bestKey K
	found := false
	src(s)(func(v T) bool {
		if k := sel(v); !found || cmp.Compare(k, bestKey) > 0 {
			best, bestKey, found = v, k, true
		}
		return true
	})
	return best, found
}

// MinBy returns the element with the smallest key; the earliest minimal
// element wins ties. ⚠ Full drain.
func (s Seq[T]) MinBy[K cmp.Ordered](sel func(T) K) (T, bool) {
	var best T
	var bestKey K
	found := false
	src(s)(func(v T) bool {
		if k := sel(v); !found || cmp.Compare(k, bestKey) < 0 {
			best, bestKey, found = v, k, true
		}
		return true
	})
	return best, found
}

// MaxOf returns the largest key. ⚠ Full drain.
func (s Seq[T]) MaxOf[K cmp.Ordered](sel func(T) K) (K, bool) {
	var best K
	found := false
	src(s)(func(v T) bool {
		if k := sel(v); !found || cmp.Compare(k, best) > 0 {
			best, found = k, true
		}
		return true
	})
	return best, found
}

// MinOf returns the smallest key. ⚠ Full drain.
func (s Seq[T]) MinOf[K cmp.Ordered](sel func(T) K) (K, bool) {
	var best K
	found := false
	src(s)(func(v T) bool {
		if k := sel(v); !found || cmp.Compare(k, best) < 0 {
			best, found = k, true
		}
		return true
	})
	return best, found
}

// MinMaxOf returns the smallest and largest keys in one pass. ⚠ Full
// drain.
func (s Seq[T]) MinMaxOf[K cmp.Ordered](sel func(T) K) (min, max K, ok bool) {
	src(s)(func(v T) bool {
		k := sel(v)
		if !ok {
			min, max, ok = k, k, true
			return true
		}
		if cmp.Compare(k, min) < 0 {
			min = k
		}
		if cmp.Compare(k, max) > 0 {
			max = k
		}
		return true
	})
	return min, max, ok
}

// MaxWith returns the largest element under cmp; the earliest maximal
// element wins ties. ⚠ Full drain.
func (s Seq[T]) MaxWith(cmp func(a, b T) int) (T, bool) {
	var best T
	found := false
	src(s)(func(v T) bool {
		if !found || cmp(v, best) > 0 {
			best, found = v, true
		}
		return true
	})
	return best, found
}

// MinWith returns the smallest element under cmp; the earliest minimal
// element wins ties. ⚠ Full drain.
func (s Seq[T]) MinWith(cmp func(a, b T) int) (T, bool) {
	var best T
	found := false
	src(s)(func(v T) bool {
		if !found || cmp(v, best) < 0 {
			best, found = v, true
		}
		return true
	})
	return best, found
}

// topEntry pairs an element with its key and encounter index for stable
// bounded-heap selection.
type topEntry[T any, K cmp.Ordered] struct {
	v   T
	k   K
	idx int
}

// topNBy is the shared bounded-heap selection behind TopNBy, BottomNBy
// and TopN. desc picks the end being kept — largest keys when true,
// smallest when false — and op names the caller in the negative-n panic,
// so a reader is pointed at the function they actually called rather than
// at this helper.
func topNBy[T any, K cmp.Ordered](op string, s Seq[T], n int, sel func(T) K, desc bool) []T {
	negCheck(op, n)
	if n == 0 {
		return nil
	}
	// Min-heap on the eviction boundary: for Top the boundary is the
	// smallest kept key, for Bottom the largest. Among equal boundary
	// keys the latest-seen sits on top so the earliest survives.
	worse := func(a, b topEntry[T, K]) bool {
		c := cmp.Compare(a.k, b.k)
		if !desc {
			c = -c
		}
		if c != 0 {
			return c < 0
		}
		return a.idx > b.idx
	}
	var h []topEntry[T, K]
	siftUp := func(i int) {
		for i > 0 {
			p := (i - 1) / 2
			if !worse(h[i], h[p]) {
				break
			}
			h[i], h[p] = h[p], h[i]
			i = p
		}
	}
	siftDown := func() {
		i := 0
		for {
			l, r := 2*i+1, 2*i+2
			m := i
			if l < len(h) && worse(h[l], h[m]) {
				m = l
			}
			if r < len(h) && worse(h[r], h[m]) {
				m = r
			}
			if m == i {
				return
			}
			h[i], h[m] = h[m], h[i]
			i = m
		}
	}
	idx := 0
	src(s)(func(v T) bool {
		e := topEntry[T, K]{v, sel(v), idx}
		idx++
		if len(h) < n {
			h = append(h, e)
			siftUp(len(h) - 1)
			return true
		}
		// Displace only on a strictly better key. Equal keys keep the
		// earlier element: e always has the largest index, so worse()
		// classifies a boundary-key tie as worse and it is skipped here.
		if worse(e, h[0]) {
			return true
		}
		h[0] = e
		siftDown()
		return true
	})
	if len(h) == 0 {
		return nil // nil-for-empty, like every slice-returning terminal
	}
	slices.SortFunc(h, func(a, b topEntry[T, K]) int {
		c := cmp.Compare(a.k, b.k)
		if desc {
			c = -c
		}
		if c != 0 {
			return c
		}
		return cmp.Compare(a.idx, b.idx)
	})
	out := make([]T, len(h))
	for i, e := range h {
		out[i] = e.v
	}
	return out
}

// TopNBy returns the n elements with the largest keys, sorted descending
// by key; equal keys retain encounter order and the earliest elements win
// at the cut. Memory is O(n) — the streaming alternative to
// SortedByDesc().Take(n). ⚠ Full drain. Panics if n is negative.
func (s Seq[T]) TopNBy[K cmp.Ordered](n int, sel func(T) K) []T {
	return topNBy("TopNBy", s, n, sel, true)
}

// BottomNBy returns the n elements with the smallest keys, sorted
// ascending by key; equal keys retain encounter order. Memory is O(n).
// ⚠ Full drain. Panics if n is negative.
func (s Seq[T]) BottomNBy[K cmp.Ordered](n int, sel func(T) K) []T {
	return topNBy("BottomNBy", s, n, sel, false)
}

// SumOf sums the selected values; integer overflow wraps like +. Empty
// input sums to 0. ⚠ Full drain.
func (s Seq[T]) SumOf[N Numeric](sel func(T) N) N {
	var sum N
	src(s)(func(v T) bool {
		sum += sel(v)
		return true
	})
	return sum
}

// ProductOf multiplies the selected values. Empty input yields 1, the
// multiplicative identity. ⚠ Full drain.
func (s Seq[T]) ProductOf[N Numeric](sel func(T) N) N {
	prod := N(1)
	src(s)(func(v T) bool {
		prod *= sel(v)
		return true
	})
	return prod
}

// AverageOf returns the mean of the selected values, accumulating in
// float64 (naive summation — precision for large integer inputs is not
// guaranteed); (0, false) on empty input. ⚠ Full drain.
func (s Seq[T]) AverageOf[N Numeric](sel func(T) N) (float64, bool) {
	var sum float64
	n := 0
	src(s)(func(v T) bool {
		sum += float64(sel(v))
		n++
		return true
	})
	if n == 0 {
		return 0, false
	}
	return sum / float64(n), true
}

// JoinToString concatenates the selected strings with sep between
// elements. ⚠ Full drain.
func (s Seq[T]) JoinToString(sep string, sel func(T) string) string {
	var b strings.Builder
	first := true
	src(s)(func(v T) bool {
		if !first {
			b.WriteString(sep)
		}
		first = false
		b.WriteString(sel(v))
		return true
	})
	return b.String()
}
