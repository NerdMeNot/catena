---
title: "Transforming"
description: "Changing what the elements are. These are the operators generic methods made possible: the element type changes mid-chain, and the chain keeps its methods. All of them stream — nothing here buffers."
sidebar:
  order: 4
---

Changing what the elements are. These are the operators generic methods made possible: the element type changes mid-chain, and the chain keeps its methods. All of them stream — nothing here buffers.

## Map

```go
func (s Seq[T]) Map[U any](f func(T) U) Seq[U]
```

Map yields f applied to each element.

```go
// The element type changes mid-chain, which is what generic methods
// made possible.
fmt.Println(catena.Of(1, 2, 3).
	Map(func(n int) string { return strings.Repeat("*", n) }).
	Collect())
```

```
[* ** ***]
```

## MapIndexed

```go
func (s Seq[T]) MapIndexed[U any](f func(int, T) U) Seq[U]
```

MapIndexed yields f(index, element), counting from 0.

```go
fmt.Println(catena.Of("a", "b", "c").
	MapIndexed(func(i int, s string) string { return fmt.Sprintf("%d:%s", i, s) }).
	Collect())
```

```
[0:a 1:b 2:c]
```

## FilterMap

```go
func (s Seq[T]) FilterMap[U any](f func(T) (U, bool)) Seq[U]
```

FilterMap yields the mapped value for each element f reports true for — a fused Map + Filter.

```go
// Fused filter and map: one stage, and the comma-ok shape means the
// mapped value is discarded rather than computed twice.
fmt.Println(catena.Of("1", "x", "3").
	FilterMap(func(s string) (int, bool) {
		n, err := strconv.Atoi(s)
		return n, err == nil
	}).
	Collect())
```

```
[1 3]
```

## FlatMap

```go
func (s Seq[T]) FlatMap[U any](f func(T) Seq[U]) Seq[U]
```

FlatMap yields all elements of f(v) for each element v, in order.

```go
fmt.Println(catena.Of(1, 2).
	FlatMap(func(n int) catena.Seq[int] { return catena.Of(n, -n) }).
	Collect())
```

```
[1 -1 2 -2]
```

## FlatMapSlice

```go
func (s Seq[T]) FlatMapSlice[U any](f func(T) []U) Seq[U]
```

FlatMapSlice yields all elements of the slice f(v) for each element v.

```go
// The same, when the callback already has a slice in hand.
fmt.Println(catena.Of("a b", "c d").
	FlatMapSlice(func(s string) []string { return strings.Fields(s) }).
	Collect())
```

```
[a b c d]
```

## Flatten

```go
func Flatten[T any](s Seq[Seq[T]]) Seq[T]
```

Flatten yields every element of every inner sequence, in order.

```go
inner := catena.Of(catena.Of(1, 2), catena.Of(3))
fmt.Println(catena.Flatten(inner).Collect())
```

```
[1 2 3]
```

## FlattenSlices

```go
func FlattenSlices[T any](s Seq[[]T]) Seq[T]
```

FlattenSlices yields every element of every slice, in order.

```go
fmt.Println(catena.FlattenSlices(catena.Of([]int{1, 2}, []int{3})).Collect())
```

```
[1 2 3]
```

## Scan

```go
func (s Seq[T]) Scan[A any](init A, f func(A, T) A) Seq[A]
```

Scan yields the running accumulator: f(init, e0), f(that, e1), ... The initial value itself is not yielded.

```go
// A running fold: the accumulator is emitted at each step. The
// initial value itself is not emitted, so the output is as long as
// the input.
fmt.Println(catena.Of(1, 2, 3, 4).
	Scan(0, func(sum, n int) int { return sum + n }).
	Collect())
```

```
[1 3 6 10]
```

## MapErr

```go
func (s Seq[T]) MapErr[U any](f func(T) (U, error)) Try[U]
```

MapErr yields f applied to each element as a Try; a failed call yields (zero, err).

```go
// A mapping that can fail produces a Try; a failed call yields the
// zero value alongside the error, never a half-built one.
parsed := catena.Of("1", "two", "3").MapErr(strconv.Atoi)
vals, errs := parsed.CollectAll()
fmt.Println(vals, len(errs))
```

```
[1 3] 1
```

## OnEach

```go
func (s Seq[T]) OnEach(f func(T)) Seq[T]
```

OnEach calls f on every element and passes it through unchanged.

```go
// Side effects without changing the stream — logging, metrics,
// progress. Elements pass through untouched.
seen := 0
total := catena.Of(1, 2, 3).
	OnEach(func(int) { seen++ }).
	SumOf(catena.Self[int])
fmt.Println(total, seen)
```

```
6 3
```
