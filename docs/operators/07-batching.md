# Batching

Grouping consecutive elements into slices — for bulk writes, paged requests, or moving windows. All three are package functions of necessity: a method on Seq[T] returning Seq[[]T] is an instantiation cycle. Every emitted slice is fresh, so retaining one is always safe.

## Chunked

```go
func Chunked[T any](s Seq[T], n int) Seq[[]T]
```

Chunked yields consecutive chunks of n elements; the last chunk may be partial. Every chunk is a fresh slice (safe to retain). Panics if n <= 0. Chunked is a package function, not a method: a method on Seq[T] returning Seq[[]T] is an instantiation cycle (each Seq[T] would require Seq[[]T], which would require Seq[[][]T], forever).

```go
// Fixed-size batches; the last one may be short. Every chunk is a
// fresh slice, so keeping one is safe.
for c := range catena.Chunked(catena.Range(1, 8, 1), 3).Seq() {
	fmt.Println(c)
}
```

```
[1 2 3]
[4 5 6]
[7]
```

## ChunkedBy

```go
func ChunkedBy[T any, K comparable](s Seq[T], sel func(T) K) Seq[[]T]
```

ChunkedBy yields runs of consecutive elements sharing a key: a chunk closes when the key changes. Memory is bounded by the longest run. Every chunk is a fresh slice. A package function for the same instantiation- cycle reason as Chunked.

```go
// A chunk per run of equal keys — the streaming answer to grouping
// already-sorted input, in memory bounded by the longest run.
for c := range catena.ChunkedBy(catena.Of(1, 1, 2, 3, 3), catena.Self[int]).Seq() {
	fmt.Println(c)
}
```

```
[1 1]
[2]
[3 3]
```

## Windowed

```go
func Windowed[T any](s Seq[T], size, step int) Seq[[]T]
```

Windowed yields sliding windows of exactly size elements, advancing by step; trailing elements that do not fill a window are dropped. step > size is valid and samples with gaps. Every window is a fresh slice. Panics if size or step is <= 0. A package function for the same instantiation-cycle reason as Chunked.

:::caution
Buffers size elements
:::

```go
// Overlapping windows: size 3 advancing by 1. Trailing elements that
// cannot fill a window are dropped.
for w := range catena.Windowed(catena.Range(1, 6, 1), 3, 1).Seq() {
	fmt.Println(w)
}
```

```
[1 2 3]
[2 3 4]
[3 4 5]
```
