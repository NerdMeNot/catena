# Slicing

Taking a window of the sequence by position or by a predicate. Take and TakeWhile consume only what they emit, so either one bounds an infinite source; TakeLast and DropLast cannot, since they have to reach the end first.

## Take

```go
func (s Seq[T]) Take(n int) Seq[T]
```

Take yields at most the first n elements, consuming exactly as many as it yields. Panics if n is negative.

```go
// Consumes exactly what it emits, so it bounds an infinite source.
fmt.Println(catena.Generate(1, func(n int) int { return n + 1 }).
	Take(3).
	Collect())
```

```
[1 2 3]
```

## TakeWhile

```go
func (s Seq[T]) TakeWhile(pred func(T) bool) Seq[T]
```

TakeWhile yields elements until pred first returns false.

```go
// Stops at the first element that fails — unlike Filter, which would
// keep testing the rest.
fmt.Println(catena.Of(1, 2, 9, 3).
	TakeWhile(func(n int) bool { return n < 5 }).
	Collect())
```

```
[1 2]
```

## TakeLast

```go
func (s Seq[T]) TakeLast(n int) Seq[T]
```

TakeLast yields the final n elements. Panics if n is negative.

:::caution
Buffers n elements and fully drains the source before emitting — hangs on infinite input
:::

```go
fmt.Println(catena.Range(1, 8, 1).TakeLast(3).Collect())
```

```
[5 6 7]
```

## Drop

```go
func (s Seq[T]) Drop(n int) Seq[T]
```

Drop skips the first n elements. Panics if n is negative.

```go
fmt.Println(catena.Of("a", "b", "c", "d").Drop(2).Collect())
```

```
[c d]
```

## DropWhile

```go
func (s Seq[T]) DropWhile(pred func(T) bool) Seq[T]
```

DropWhile skips elements until pred first returns false, then yields the rest.

```go
// Drops only the leading run; once the predicate fails, everything
// after is kept.
fmt.Println(catena.Of(0, 0, 3, 0, 5).
	DropWhile(func(n int) bool { return n == 0 }).
	Collect())
```

```
[3 0 5]
```

## DropLast

```go
func (s Seq[T]) DropLast(n int) Seq[T]
```

DropLast yields all but the final n elements, emitting with an n-element lag. Panics if n is negative.

:::caution
Buffers n elements
:::

```go
fmt.Println(catena.Range(1, 6, 1).DropLast(2).Collect())
```

```
[1 2 3]
```

## Step

```go
func (s Seq[T]) Step(n int) Seq[T]
```

Step yields the first element and every nth element after it. Panics if n <= 0.

```go
// The first element always survives, then every nth after it.
fmt.Println(catena.Range(0, 10, 1).Step(3).Collect())
```

```
[0 3 6 9]
```
