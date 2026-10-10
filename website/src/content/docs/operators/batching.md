---
title: "Batching"
description: "Grouping consecutive elements into slices — for bulk writes, paged requests, or moving windows. All three are package functions of necessity: a method on Seq[T] returning Seq[[]T] is an instantiation cycle. Every emitted slice is fresh, so retaining one is always safe."
sidebar:
  order: 7
---

Grouping consecutive elements into slices — for bulk writes, paged requests, or moving windows. All three are package functions of necessity: a method on Seq[T] returning Seq[[]T] is an instantiation cycle. Every emitted slice is fresh, so retaining one is always safe.

## Chunked

```go
func Chunked[T any](s Seq[T], n int) Seq[[]T]
```

Chunked yields consecutive chunks of n elements; the last chunk may be partial. Every chunk is a fresh slice (safe to retain). Panics if n <= 0. Chunked is a package function, not a method: a method on Seq[T] returning Seq[[]T] is an instantiation cycle (each Seq[T] would require Seq[[]T], which would require Seq[[][]T], forever).

```go
// Fixed-size batches, for an API that takes so many IDs per request;
// the last one may be short. Every chunk is a fresh slice, so keeping
// one is safe.
userIDs := catena.Range(101, 108, 1)
for batch := range catena.Chunked(userIDs, 3).Seq() {
	fmt.Println("DELETE /users", batch)
}
```

```
DELETE /users [101 102 103]
DELETE /users [104 105 106]
DELETE /users [107]
```

## ChunkedBy

```go
func ChunkedBy[T any, K comparable](s Seq[T], sel func(T) K) Seq[[]T]
```

ChunkedBy yields runs of consecutive elements sharing a key: a chunk closes when the key changes. Memory is bounded by the longest run. Every chunk is a fresh slice. A package function for the same instantiation- cycle reason as Chunked.

```go
// A chunk per run of equal keys — the streaming answer to grouping
// input that is already ordered, in memory bounded by the longest run.
// It splits on change, not on value: a host that reappears later
// starts a new chunk.
type entry struct {
	Host string
	Path string
}
access := catena.Of(
	entry{"web-1", "/"}, entry{"web-1", "/cart"},
	entry{"web-2", "/"},
	entry{"web-1", "/pay"},
)
for run := range catena.ChunkedBy(access, func(e entry) string { return e.Host }).Seq() {
	fmt.Println(run[0].Host, len(run))
}
```

```
web-1 2
web-2 1
web-1 1
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
// Overlapping windows: size 3 advancing by 1, which is a moving
// average — the spike is smoothed in rather than jumping. Only full
// windows are yielded, so five samples make three.
latencies := catena.Of(120, 130, 110, 400, 420)
for w := range catena.Windowed(latencies, 3, 1).Seq() {
	fmt.Println(w, "avg", catena.Sum(catena.FromSlice(w))/len(w))
}
```

```
[120 130 110] avg 120
[130 110 400] avg 213
[110 400 420] avg 310
```
