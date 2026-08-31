package catena

// The batch operators. All three are package functions of necessity, not
// style: a method on Seq[T] returning Seq[[]T] is an instantiation cycle —
// each Seq[T] would require Seq[[]T]'s method set, which would require
// Seq[[][]T]'s, forever. Every emitted slice is fresh;
// retaining one is always safe (implementation law L3).

import "slices"

// Chunked yields consecutive chunks of n elements; the last chunk may be
// partial. Every chunk is a fresh slice (safe to retain). Panics if n <= 0.
//
// Chunked is a package function, not a method: a method on Seq[T]
// returning Seq[[]T] is an instantiation cycle (each Seq[T] would require
// Seq[[]T], which would require Seq[[][]T], forever).
func Chunked[T any](s Seq[T], n int) Seq[[]T] {
	posCheck("Chunked", "size", n)
	return func(yield func([]T) bool) {
		chunk := make([]T, 0, n)
		stopped := false
		src(s)(func(v T) bool {
			chunk = append(chunk, v)
			if len(chunk) < n {
				return true
			}
			if !yield(chunk) {
				stopped = true
				return false
			}
			chunk = make([]T, 0, n) // fresh: never reuse an emitted slice
			return true
		})
		if !stopped && len(chunk) > 0 {
			yield(chunk)
		}
	}
}

// ChunkedBy yields runs of consecutive elements sharing a key: a chunk
// closes when the key changes. Memory is bounded by the longest run. Every
// chunk is a fresh slice. A package function for the same instantiation-
// cycle reason as Chunked.
func ChunkedBy[T any, K comparable](s Seq[T], sel func(T) K) Seq[[]T] {
	return func(yield func([]T) bool) {
		var chunk []T
		var prev K
		stopped := false
		src(s)(func(v T) bool {
			k := sel(v)
			if len(chunk) > 0 && k != prev {
				if !yield(chunk) {
					stopped = true
					return false
				}
				chunk = nil // fresh: never reuse an emitted slice
			}
			chunk = append(chunk, v)
			prev = k
			return true
		})
		if !stopped && len(chunk) > 0 {
			yield(chunk)
		}
	}
}

// Windowed yields sliding windows of exactly size elements, advancing by
// step; trailing elements that do not fill a window are dropped. step >
// size is valid and samples with gaps. ⚠ Buffers size elements. Every
// window is a fresh slice. Panics if size or step is <= 0. A package
// function for the same instantiation-cycle reason as Chunked.
func Windowed[T any](s Seq[T], size, step int) Seq[[]T] {
	posCheck("Windowed", "size", size)
	posCheck("Windowed", "step", step)
	return func(yield func([]T) bool) {
		win := make([]T, 0, size)
		skip := 0
		src(s)(func(v T) bool {
			if skip > 0 {
				skip--
				return true
			}
			win = append(win, v)
			if len(win) < size {
				return true
			}
			if !yield(slices.Clone(win)) {
				return false
			}
			if step >= size {
				win = win[:0]
				skip = step - size
			} else {
				win = append(win[:0], win[step:]...)
			}
			return true
		})
	}
}
