---
title: "Folding and grouping"
description: "Reducing a sequence to one value, or to one value per key. FoldBy is the operator that justifies the library: it aggregates per key as elements stream past, so its memory is bounded by the number of distinct keys rather than by the number of elements — where GroupBy retains everything."
sidebar:
  order: 10
---

Reducing a sequence to one value, or to one value per key. FoldBy is the operator that justifies the library: it aggregates per key as elements stream past, so its memory is bounded by the number of distinct keys rather than by the number of elements — where GroupBy retains everything.

## Fold

```go
func (s Seq[T]) Fold[A any](init A, f func(A, T) A) A
```

Fold reduces the sequence into an accumulator, left to right. Reduce is the same operation using the first element as the initial accumulator; FoldBy folds per key in one streaming pass.

:::caution
Full drain
:::

```go
fmt.Println(catena.Of(1, 2, 3).Fold(100, func(acc, n int) int { return acc + n }))
```

```
106
```

## FoldIndexed

```go
func (s Seq[T]) FoldIndexed[A any](init A, f func(int, A, T) A) A
```

FoldIndexed is Fold with the element index.

:::caution
Full drain
:::

```go
fmt.Println(catena.Of(10, 20).FoldIndexed(0, func(i, acc, n int) int { return acc + i*n }))
```

```
20
```

## FoldWhile

```go
func (s Seq[T]) FoldWhile[A any](init A, f func(A, T) (A, bool)) A
```

FoldWhile folds until f reports false; the accumulator from the stopping call is included in the result.

```go
// Stops when the callback says so; the accumulator from the stopping
// call is included.
fmt.Println(catena.Of(1, 2, 3, 4).FoldWhile(0, func(acc, n int) (int, bool) {
	acc += n
	return acc, acc < 5
}))
```

```
6
```

## FoldErr

```go
func (s Seq[T]) FoldErr[A any](init A, f func(A, T) (A, error)) (A, error)
```

FoldErr folds until f fails, returning the accumulator so far and the first error.

```go
// Stops at the first error and returns the accumulator so far.
acc, err := catena.Of(1, 2, 3).FoldErr(0, func(acc, n int) (int, error) {
	if n == 3 {
		return 0, errors.New("three is too many")
	}
	return acc + n, nil
})
fmt.Println(acc, err)
```

```
3 three is too many
```

## FoldBy

```go
func (s Seq[T]) FoldBy[K comparable, A any](
	key func(T) K, init func(K) A, f func(A, T) A,
) map[K]A
```

FoldBy folds each element into a per-key accumulator: streaming grouped aggregation with no intermediate per-key slices. init is called once per distinct key. Memory is bounded by the number of distinct keys, not elements.

:::caution
Full drain; map iteration order is undefined
:::

```go
// Streaming aggregation per key. GroupBy would retain every element;
// this retains one accumulator per distinct key.
type sale struct {
	Region string
	Amount int
}
sales := catena.Of(
	sale{"west", 100}, sale{"east", 40}, sale{"west", 20},
)
totals := sales.FoldBy(
	func(s sale) string { return s.Region },
	func(string) int { return 0 },
	func(sum int, s sale) int { return sum + s.Amount },
)
fmt.Println(totals["west"], totals["east"])
```

```
120 40
```

## Reduce

```go
func (s Seq[T]) Reduce(f func(T, T) T) (T, bool)
```

Reduce folds the sequence using its first element as the initial accumulator; (zero, false) on empty input.

:::caution
Full drain
:::

```go
// Uses the first element as the seed, so it reports false on empty
// rather than inventing a zero.
v, ok := catena.Of(3, 1, 2).Reduce(func(a, b int) int { return a * b })
_, empty := catena.Empty[int]().Reduce(func(a, b int) int { return a })
fmt.Println(v, ok, empty)
```

```
6 true false
```

## GroupBy

```go
func (s Seq[T]) GroupBy[K comparable](sel func(T) K) map[K][]T
```

GroupBy collects elements into per-key buckets, each in encounter order.

:::caution
Full drain; retains every element; map iteration order is undefined
:::

```go
// Retains every element. For an aggregate, FoldBy is bounded by keys.
byParity := catena.Range(1, 6, 1).GroupBy(func(n int) string {
	if n%2 == 0 {
		return "even"
	}
	return "odd"
})
fmt.Println(byParity["odd"], byParity["even"])
```

```
[1 3 5] [2 4]
```

## IndexBy

```go
func (s Seq[T]) IndexBy[K comparable](sel func(T) K) map[K]T
```

IndexBy maps each key to its element; on duplicate keys the last element wins.

:::caution
Full drain; map iteration order is undefined
:::

```go
// A lookup table; on a duplicate key the last element wins.
m := catena.Of("apple", "avocado", "blueberry").
	IndexBy(func(s string) byte { return s[0] })
fmt.Println(string(m['a']), string(m['b']))
```

```
avocado blueberry
```

## TallyBy

```go
func (s Seq[T]) TallyBy[K comparable](sel func(T) K) map[K]int
```

TallyBy counts elements per key.

:::caution
Full drain; map iteration order is undefined
:::

```go
counts := catena.Of("apple", "avocado", "blueberry").
	TallyBy(func(s string) byte { return s[0] })
fmt.Println(counts['a'], counts['b'])
```

```
2 1
```

## Associate

```go
func (s Seq[T]) Associate[K comparable, V any](f func(T) (K, V)) map[K]V
```

Associate builds a map from f's key/value pairs; on duplicate keys the last pair wins.

:::caution
Full drain; map iteration order is undefined
:::

```go
m := catena.Of(1, 2).Associate(func(n int) (int, string) {
	return n, strings.Repeat("*", n)
})
fmt.Println(m[1], m[2])
```

```
* **
```

## Partition

```go
func (s Seq[T]) Partition(pred func(T) bool) (yes, no []T)
```

Partition splits elements by pred, preserving encounter order on both sides; nil slices for empty sides.

:::caution
Full drain
:::

```go
// Both sides in one pass, each in encounter order.
even, odd := catena.Range(1, 6, 1).Partition(func(n int) bool { return n%2 == 0 })
fmt.Println(even, odd)
```

```
[2 4] [1 3 5]
```

## AssociateWith

```go
func AssociateWith[T comparable, V any](s Seq[T], f func(T) V) map[T]V
```

AssociateWith maps each element to f(element); on duplicate elements the last value wins.

:::caution
Full drain; map iteration order is undefined
:::

```go
m := catena.AssociateWith(catena.Of("go", "rust"), func(s string) int { return len(s) })
fmt.Println(m["go"], m["rust"])
```

```
2 4
```

## Tally

```go
func Tally[T comparable](s Seq[T]) map[T]int
```

Tally counts occurrences per value.

:::caution
Full drain; map iteration order is undefined
:::

```go
fmt.Println(catena.Tally(catena.Of("a", "b", "a"))["a"])
```

```
2
```

## ToKeySet

```go
func ToKeySet[T comparable](s Seq[T]) map[T]struct{}
```

ToKeySet drains the sequence into a membership map, whose keys are the distinct elements. Named for what it returns rather than for the set it stands in for: ToSet is reserved for a real set type, should Go ever grow one.

:::caution
Full drain; map iteration order is undefined
:::

```go
set := catena.ToKeySet(catena.Of(1, 2, 2))
_, has := set[2]
fmt.Println(len(set), has)
```

```
2 true
```
