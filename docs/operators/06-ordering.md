# Ordering

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

SortedBy yields the elements sorted ascending by key, stably. sel is called exactly once per element (decorate-sort-undecorate).

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
