# Searching and testing

Asking a question of a sequence rather than transforming it. The short-circuiting ones — First, Find, Any, All, None — stop as soon as the answer is settled, so they terminate on an infinite source; Last, Count and FindLast cannot, because the answer depends on the final element.

## First

```go
func (s Seq[T]) First() (T, bool)
```

First returns the first element.

```go
v, ok := catena.Of(3, 1).First()
_, empty := catena.Empty[int]().First()
fmt.Println(v, ok, empty)
```

```
3 true false
```

## Last

```go
func (s Seq[T]) Last() (T, bool)
```

Last returns the final element.

:::caution
Full drain
:::

```go
v, ok := catena.Of(3, 1).Last()
fmt.Println(v, ok)
```

```
1 true
```

## Single

```go
func (s Seq[T]) Single() (T, bool)
```

Single returns the element iff the sequence has exactly one; it stops consuming upon seeing a second.

```go
// True only for exactly one element; it stops as soon as a second
// arrives rather than counting the rest.
a, ok1 := catena.Of(7).Single()
_, ok2 := catena.Of(7, 8).Single()
fmt.Println(a, ok1, ok2)
```

```
7 true false
```

## ElementAt

```go
func (s Seq[T]) ElementAt(i int) (T, bool)
```

ElementAt returns the element at index i; (zero, false) for a negative or out-of-range index.

```go
v, ok := catena.Of("a", "b", "c").ElementAt(1)
_, neg := catena.Of("a").ElementAt(-1)
fmt.Println(v, ok, neg)
```

```
b true false
```

## Find

```go
func (s Seq[T]) Find(pred func(T) bool) (T, bool)
```

Find returns the first element pred admits.

```go
v, ok := catena.Of(1, 4, 9).Find(func(n int) bool { return n > 3 })
fmt.Println(v, ok)
```

```
4 true
```

## FindLast

```go
func (s Seq[T]) FindLast(pred func(T) bool) (T, bool)
```

FindLast returns the final element pred admits.

:::caution
Full drain
:::

```go
v, ok := catena.Of(1, 4, 9).FindLast(func(n int) bool { return n > 3 })
fmt.Println(v, ok)
```

```
9 true
```

## FindIndex

```go
func (s Seq[T]) FindIndex(pred func(T) bool) int
```

FindIndex returns the index of the first element pred admits; -1 if none.

```go
fmt.Println(catena.Of("a", "b").FindIndex(func(s string) bool { return s == "b" }))
fmt.Println(catena.Of("a").FindIndex(func(s string) bool { return s == "z" }))
```

```
1
-1
```

## FindMap

```go
func (s Seq[T]) FindMap[U any](f func(T) (U, bool)) (U, bool)
```

FindMap returns the first mapped value f reports true for — a fused Find + Map.

```go
// Fused find and map: the mapped value is returned, not the element.
v, ok := catena.Of("x", "12", "y").FindMap(func(s string) (int, bool) {
	n := 0
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err == nil
})
fmt.Println(v, ok)
```

```
12 true
```

## Any

```go
func (s Seq[T]) Any(pred func(T) bool) bool
```

Any reports whether pred admits any element; stops at the first match.

```go
// Stops at the first match, so it terminates on an infinite source.
fmt.Println(catena.Generate(1, func(n int) int { return n + 1 }).
	Any(func(n int) bool { return n > 100 }))
```

```
true
```

## All

```go
func (s Seq[T]) All(pred func(T) bool) bool
```

All reports whether pred admits every element; stops at the first counterexample. Vacuously true on empty input.

```go
// Vacuously true on an empty sequence.
fmt.Println(catena.Of(2, 4).All(func(n int) bool { return n%2 == 0 }))
fmt.Println(catena.Empty[int]().All(func(n int) bool { return false }))
```

```
true
true
```

## None

```go
func (s Seq[T]) None(pred func(T) bool) bool
```

None reports whether pred admits no element; stops at the first match.

```go
fmt.Println(catena.Of(1, 3).None(func(n int) bool { return n%2 == 0 }))
```

```
true
```

## Count

```go
func (s Seq[T]) Count() int
```

Count returns the number of elements.

:::caution
Full drain
:::

```go
fmt.Println(catena.Of("a", "b", "c").Count())
```

```
3
```

## CountWhere

```go
func (s Seq[T]) CountWhere(pred func(T) bool) int
```

CountWhere returns the number of elements pred admits.

:::caution
Full drain
:::

```go
// Fused filter and count: one stage rather than two.
fmt.Println(catena.Range(1, 11, 1).CountWhere(func(n int) bool { return n%3 == 0 }))
```

```
3
```

## IsEmpty

```go
func (s Seq[T]) IsEmpty() bool
```

IsEmpty reports whether the sequence yields nothing.

:::caution
It does so by consuming one element — on a single-pass source that element is lost
:::

```go
// Answers by consuming one element — on a single-pass source that
// element is gone.
fmt.Println(catena.Empty[int]().IsEmpty(), catena.Of(1).IsEmpty())
```

```
true false
```

## Contains

```go
func Contains[T comparable](s Seq[T], v T) bool
```

Contains reports whether v occurs in s; stops at the first match.

```go
fmt.Println(catena.Contains(catena.Of(1, 2, 3), 2))
```

```
true
```

## IndexOf

```go
func IndexOf[T comparable](s Seq[T], v T) int
```

IndexOf returns the index of the first occurrence of v; -1 if none.

```go
fmt.Println(catena.IndexOf(catena.Of("a", "b"), "b"))
fmt.Println(catena.IndexOf(catena.Of("a"), "z"))
```

```
1
-1
```

## Equal

```go
func Equal[T comparable](a, b Seq[T]) bool
```

Equal reports whether a and b yield the same elements in the same order. Consumes both sequences up to and including the first difference — fully when they are equal. b is consumed through iter.Pull (its cleanup always runs).

```go
fmt.Println(catena.Equal(catena.Of(1, 2), catena.Of(1, 2)))
fmt.Println(catena.Equal(catena.Of(1, 2), catena.Of(1)))
```

```
true
false
```
