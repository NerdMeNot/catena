---
title: "Aggregating"
description: "Extremes, totals and joins. Two conventions run through this page: -By returns the element with the extreme key while -Of returns the key itself, and ties always go to the first element seen. NaN orders below everything, so these agree with the sorts. TopNBy is the one to reach for on a large scan — a bounded heap rather than a full sort."
sidebar:
  order: 11
---

Extremes, totals and joins. Two conventions run through this page: -By returns the element with the extreme key while -Of returns the key itself, and ties always go to the first element seen. NaN orders below everything, so these agree with the sorts. TopNBy is the one to reach for on a large scan — a bounded heap rather than a full sort.

## MaxBy

```go
func (s Seq[T]) MaxBy[K cmp.Ordered](sel func(T) K) (T, bool)
```

MaxBy returns the element with the largest key; the earliest maximal element wins ties. NaN keys order below everything (cmp.Compare).

:::caution
Full drain
:::

```go
// Returns the ELEMENT with the largest key; ties go to the first.
type run struct {
	Name string
	Secs int
}
runs := catena.Of(run{"ada", 42}, run{"bob", 51}, run{"eve", 51})
slowest, _ := runs.MaxBy(func(r run) int { return r.Secs })
fmt.Println(slowest.Name)
```

```
bob
```

## MinBy

```go
func (s Seq[T]) MinBy[K cmp.Ordered](sel func(T) K) (T, bool)
```

MinBy returns the element with the smallest key; the earliest minimal element wins ties.

:::caution
Full drain
:::

```go
type run struct {
	Name string
	Secs int
}
runs := catena.Of(run{"ada", 42}, run{"bob", 51})
fastest, _ := runs.MinBy(func(r run) int { return r.Secs })
fmt.Println(fastest.Name)
```

```
ada
```

## MaxOf

```go
func (s Seq[T]) MaxOf[K cmp.Ordered](sel func(T) K) (K, bool)
```

MaxOf returns the largest key.

:::caution
Full drain
:::

```go
// Returns the KEY, where MaxBy returns the element — the -By/-Of
// distinction, which holds across the whole library.
longest, _ := catena.Of("go", "rust", "c").MaxOf(func(s string) int { return len(s) })
fmt.Println(longest)
```

```
4
```

## MinOf

```go
func (s Seq[T]) MinOf[K cmp.Ordered](sel func(T) K) (K, bool)
```

MinOf returns the smallest key.

:::caution
Full drain
:::

```go
shortest, _ := catena.Of("go", "rust", "c").MinOf(func(s string) int { return len(s) })
fmt.Println(shortest)
```

```
1
```

## MinMaxOf

```go
func (s Seq[T]) MinMaxOf[K cmp.Ordered](sel func(T) K) (min, max K, ok bool)
```

MinMaxOf returns the smallest and largest keys in one pass.

:::caution
Full drain
:::

```go
// Both ends in a single pass.
lo, hi, ok := catena.Of("go", "rust", "c").MinMaxOf(func(s string) int { return len(s) })
fmt.Println(lo, hi, ok)
```

```
1 4 true
```

## MaxWith

```go
func (s Seq[T]) MaxWith(cmp func(a, b T) int) (T, bool)
```

MaxWith returns the largest element under cmp; the earliest maximal element wins ties.

:::caution
Full drain
:::

```go
// A comparator, for orderings no single key expresses.
v, _ := catena.Of("bb", "a", "cccc").
	MaxWith(func(x, y string) int { return len(x) - len(y) })
fmt.Println(v)
```

```
cccc
```

## MinWith

```go
func (s Seq[T]) MinWith(cmp func(a, b T) int) (T, bool)
```

MinWith returns the smallest element under cmp; the earliest minimal element wins ties.

:::caution
Full drain
:::

```go
v, _ := catena.Of("bb", "a", "cccc").
	MinWith(func(x, y string) int { return len(x) - len(y) })
fmt.Println(v)
```

```
a
```

## TopNBy

```go
func (s Seq[T]) TopNBy[K cmp.Ordered](n int, sel func(T) K) []T
```

TopNBy returns the n elements with the largest keys, sorted descending by key; equal keys retain encounter order and the earliest elements win at the cut. Memory is O(n) — the streaming alternative to SortedByDesc().Take(n). Panics if n is negative.

:::caution
Full drain
:::

```go
// A bounded heap of n, not a sort: memory is O(n) rather than O(all),
// which is the difference between a working pipeline and an OOM on a
// large scan. Output is sorted descending, ties in encounter order.
fmt.Println(catena.Of(5, 1, 9, 3, 7).TopNBy(3, catena.Self[int]))
```

```
[9 7 5]
```

## BottomNBy

```go
func (s Seq[T]) BottomNBy[K cmp.Ordered](n int, sel func(T) K) []T
```

BottomNBy returns the n elements with the smallest keys, sorted ascending by key; equal keys retain encounter order. Memory is O(n). Panics if n is negative.

:::caution
Full drain
:::

```go
fmt.Println(catena.Of(5, 1, 9, 3, 7).BottomNBy(2, catena.Self[int]))
```

```
[1 3]
```

## Max

```go
func Max[T cmp.Ordered](s Seq[T]) (T, bool)
```

Max returns the largest element; NaN orders below everything (cmp.Compare).

:::caution
Full drain
:::

```go
v, ok := catena.Max(catena.Of(3, 9, 1))
fmt.Println(v, ok)
```

```
9 true
```

## Min

```go
func Min[T cmp.Ordered](s Seq[T]) (T, bool)
```

Min returns the smallest element; NaN orders below everything, so a NaN in the input is the minimum.

:::caution
Full drain
:::

```go
v, ok := catena.Min(catena.Of(3, 9, 1))
fmt.Println(v, ok)
```

```
1 true
```

## MinMax

```go
func MinMax[T cmp.Ordered](s Seq[T]) (min, max T, ok bool)
```

MinMax returns the smallest and largest elements in one pass.

:::caution
Full drain
:::

```go
lo, hi, ok := catena.MinMax(catena.Of(3, 9, 1))
fmt.Println(lo, hi, ok)
```

```
1 9 true
```

## TopN

```go
func TopN[T cmp.Ordered](s Seq[T], n int) []T
```

TopN returns the n largest elements, sorted descending; equal elements retain encounter order. Memory is O(n). Panics if n is negative.

:::caution
Full drain
:::

```go
fmt.Println(catena.TopN(catena.Of(5, 1, 9, 3), 2))
```

```
[9 5]
```

## SumOf

```go
func (s Seq[T]) SumOf[N Numeric](sel func(T) N) N
```

SumOf sums the selected values; integer overflow wraps like +. Empty input sums to 0.

:::caution
Full drain
:::

```go
type item struct{ Qty int }
items := catena.Of(item{2}, item{3})
fmt.Println(items.SumOf(func(i item) int { return i.Qty }))
```

```
5
```

## ProductOf

```go
func (s Seq[T]) ProductOf[N Numeric](sel func(T) N) N
```

ProductOf multiplies the selected values. Empty input yields 1, the multiplicative identity.

:::caution
Full drain
:::

```go
// The empty product is 1, the multiplicative identity.
fmt.Println(catena.Of(2, 3, 4).ProductOf(catena.Self[int]))
fmt.Println(catena.Empty[int]().ProductOf(catena.Self[int]))
```

```
24
1
```

## AverageOf

```go
func (s Seq[T]) AverageOf[N Numeric](sel func(T) N) (float64, bool)
```

AverageOf returns the mean of the selected values, accumulating in float64 (naive summation — precision for large integer inputs is not guaranteed); (0, false) on empty input.

:::caution
Full drain
:::

```go
// Accumulates in float64, and reports false on empty rather than
// dividing by zero.
avg, ok := catena.Of(1, 2, 4).AverageOf(catena.Self[int])
_, empty := catena.Empty[int]().AverageOf(catena.Self[int])
fmt.Printf("%.2f %v %v\n", avg, ok, empty)
```

```
2.33 true false
```

## Sum

```go
func Sum[T Numeric](s Seq[T]) T
```

Sum adds the elements; integer overflow wraps like +. Empty input sums to 0.

:::caution
Full drain
:::

```go
// Integer overflow wraps, exactly as + does.
fmt.Println(catena.Sum(catena.Of(1, 2, 3)))
```

```
6
```

## Product

```go
func Product[T Numeric](s Seq[T]) T
```

Product multiplies the elements. Empty input yields 1, the multiplicative identity.

:::caution
Full drain
:::

```go
fmt.Println(catena.Product(catena.Of(2, 3, 4)))
```

```
24
```

## Average

```go
func Average[T Numeric](s Seq[T]) (float64, bool)
```

Average returns the mean, accumulating in float64; (0, false) on empty input.

:::caution
Full drain
:::

```go
avg, ok := catena.Average(catena.Of(2.0, 4.0))
fmt.Println(avg, ok)
```

```
3 true
```

## JoinToString

```go
func (s Seq[T]) JoinToString(sep string, sel func(T) string) string
```

JoinToString concatenates the selected strings with sep between elements.

:::caution
Full drain
:::

```go
type user struct{ Name string }
users := catena.Of(user{"ada"}, user{"bob"})
fmt.Println(users.JoinToString(", ", func(u user) string { return u.Name }))
```

```
ada, bob
```

## Join

```go
func Join(s Seq[string], sep string) string
```

Join concatenates a string sequence with sep between elements.

:::caution
Full drain
:::

```go
fmt.Println(catena.Join(catena.Of("a", "b", "c"), "-"))
```

```
a-b-c
```
