---
title: "Ordering"
description: "Sorting and reversing. Every sort here is stable, and every one buffers the whole sequence — they cannot emit a first element until they have seen the last, so none of them terminates on an infinite source."
sidebar:
  order: 6
---

Sorting and reversing. Every sort here is stable, and every one buffers the whole sequence — they cannot emit a first element until they have seen the last, so none of them terminates on an infinite source.

## Sorted

```go
func Sorted[T cmp.Ordered](s Seq[T]) Seq[T]
```

Sorted yields the elements in ascending order, stably. NaN sorts first.

:::caution
Buffers the entire input
:::

```go
fmt.Println(catena.Sorted(catena.Of(3, 1, 2)).Collect())
```

```
[1 2 3]
```

## SortedDesc

```go
func SortedDesc[T cmp.Ordered](s Seq[T]) Seq[T]
```

SortedDesc yields the elements in descending order, stably.

:::caution
Buffers the entire input
:::

```go
fmt.Println(catena.SortedDesc(catena.Of(3, 1, 2)).Collect())
```

```
[3 2 1]
```

## SortedBy

```go
func (s Seq[T]) SortedBy[K cmp.Ordered](sel func(T) K) Seq[T]
```

SortedBy yields the elements sorted ascending by key, stably. sel is called exactly once per element (decorate-sort-undecorate). catena.Sorted(s) is the no-selector form, for an ordered T.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
// Stable, and the selector runs exactly once per element rather than
// once per comparison — so an expensive key is affordable. Stability
// shows here: kiwi and date are both 4 long, and kiwi came first.
words := catena.Of("kiwi", "fig", "banana", "date")
fmt.Println(words.SortedBy(func(s string) int { return len(s) }).Collect())
```

```
[fig kiwi date banana]
```

## SortedByDesc

```go
func (s Seq[T]) SortedByDesc[K cmp.Ordered](sel func(T) K) Seq[T]
```

SortedByDesc yields the elements sorted descending by key, stably. sel is called exactly once per element.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
words := catena.Of("kiwi", "fig", "banana")
fmt.Println(words.SortedByDesc(func(s string) int { return len(s) }).Collect())
```

```
[banana kiwi fig]
```

## SortedWith

```go
func (s Seq[T]) SortedWith(cmp func(a, b T) int) Seq[T]
```

SortedWith yields the elements sorted by cmp, stably.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
// A comparator, for orderings a single key cannot express.
fmt.Println(catena.Of("b", "A", "c").
	SortedWith(func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) }).
	Collect())
```

```
[A b c]
```

## Reversed

```go
func (s Seq[T]) Reversed() Seq[T]
```

Reversed yields the elements in reverse order.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
fmt.Println(catena.Of(1, 2, 3).Reversed().Collect())
```

```
[3 2 1]
```

## IsSorted

```go
func IsSorted[T cmp.Ordered](s Seq[T]) bool
```

IsSorted reports whether the elements are in ascending order; an empty or one-element sequence is sorted. NaN orders below everything (cmp.Compare), as it does for Sorted. It stops at the first pair out of order, so on infinite input it returns only if there is one.

```go
// One pass, no buffer, and it stops at the first pair out of order.
fmt.Println(catena.IsSorted(catena.Of(1, 2, 2, 5)), catena.IsSorted(catena.Of(1, 3, 2)))
```

```
true false
```

## IsSortedBy

```go
func (s Seq[T]) IsSortedBy[K cmp.Ordered](sel func(T) K) bool
```

IsSortedBy reports whether no element's key orders before its predecessor's; an empty or one-element sequence is sorted. sel is called once per element. It stops at the first pair out of order, so on infinite input it returns only if there is one.

```go
words := catena.Of("go", "zig", "rust", "kotlin")
fmt.Println(words.IsSortedBy(func(w string) int { return len(w) }))
```

```
true
```

## IsSortedWith

```go
func (s Seq[T]) IsSortedWith(cmp func(a, b T) int) bool
```

IsSortedWith reports whether no element orders before its predecessor under cmp; an empty or one-element sequence is sorted. It stops at the first pair out of order, so on infinite input it returns only if there is one. catena.IsSorted is the no-comparator form, for an ordered T.

```go
// Any ordering: here, descending.
scores := catena.Of(90, 85, 85, 70)
fmt.Println(scores.IsSortedWith(func(a, b int) int { return b - a }))
```

```
true
```
