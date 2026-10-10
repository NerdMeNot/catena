---
title: "Consuming"
description: "The operators that make a pipeline run. Everything before one of these is a description of work; one of these performs it, once. Collect and ForEach drain the sequence, so neither returns on an infinite one — Pull and ToChan hand control back to you instead, and both make you responsible for stopping."
sidebar:
  order: 8
---

The operators that make a pipeline run. Everything before one of these is a description of work; one of these performs it, once. Collect and ForEach drain the sequence, so neither returns on an infinite one — Pull and ToChan hand control back to you instead, and both make you responsible for stopping.

## Collect

```go
func (s Seq[T]) Collect() []T
```

Collect drains the sequence into a slice; nil for empty. ToList is the same drain returning a List, which carries the eager operator set.

:::caution
Full drain
:::

```go
// nil for an empty sequence, matching slices.Collect.
fmt.Println(catena.Of(1, 2).Collect(), catena.Empty[int]().Collect() == nil)
```

```
[1 2] true
```

## ToList

```go
func (s Seq[T]) ToList() List[T]
```

ToList drains the sequence into a List; nil for empty.

:::caution
Full drain
:::

```go
// The eager twin: a List has the same operations, evaluated at once.
l := catena.Of(3, 1, 2).ToList()
fmt.Println(l.Len(), l.At(0))
```

```
3 3
```

## Seq

```go
func (s Seq[T]) Seq() iter.Seq[T]
```

Seq converts to the stdlib iterator type. Free.

```go
// A free conversion to the standard iterator type, in both
// directions — Seq IS iter.Seq underneath.
fmt.Println(slices.Collect(catena.Of(1, 2, 3).Seq()))
```

```
[1 2 3]
```

## Pull

```go
func (s Seq[T]) Pull() (next func() (T, bool), stop func())
```

Pull converts s to a pull-based iterator. THE CALLER MUST CALL stop, even if next has returned false, or resources held by s will leak.

```go
// Inverts control for hand-written loops. THE CALLER MUST CALL stop,
// or a producer holding a resource never releases it.
next, stop := catena.Of("a", "b").Pull()
defer stop()

v, ok := next()
fmt.Println(v, ok)
```

```
a true
```

## ToChan

```go
func (s Seq[T]) ToChan(ctx context.Context) <-chan T
```

ToChan starts a goroutine that sends every element on the returned unbuffered channel. The channel is closed when the sequence ends or ctx is done — the consumer must drain or cancel, or the goroutine leaks.

```go
// The fan-out mechanism: a Seq is not safe to consume from two
// goroutines, a channel is. Cancelling ctx closes the channel.
for v := range catena.Of(1, 2, 3).ToChan(context.Background()) {
	fmt.Print(v, " ")
}
```

```
1 2 3
```

## ForEach

```go
func (s Seq[T]) ForEach(f func(T))
```

ForEach calls f on every element.

:::caution
Full drain
:::

```go
catena.Of("a", "b").ForEach(func(s string) { fmt.Print(s) })
```

```
ab
```

## ForEachIndexed

```go
func (s Seq[T]) ForEachIndexed(f func(int, T))
```

ForEachIndexed calls f(index, element) on every element.

:::caution
Full drain
:::

```go
catena.Of("a", "b").ForEachIndexed(func(i int, s string) { fmt.Printf("%d=%s ", i, s) })
```

```
0=a 1=b
```

## ForEachErr

```go
func (s Seq[T]) ForEachErr(f func(T) error) error
```

ForEachErr calls f on every element, stopping at and returning the first non-nil error; nil if the sequence drains clean.

```go
// Stops at the first error the callback returns, and returns it.
err := catena.Of(1, 2, 3).ForEachErr(func(n int) error {
	if n == 2 {
		return errors.New("stopped at 2")
	}
	fmt.Println("handled", n)
	return nil
})
fmt.Println("err:", err)
```

```
handled 1
err: stopped at 2
```

## Drain

```go
func (s Seq[T]) Drain()
```

Drain consumes the sequence for its side effects.

:::caution
Full drain
:::

```go
// Consume for side effects alone, discarding the elements.
count := 0
catena.Of(1, 2, 3).OnEach(func(int) { count++ }).Drain()
fmt.Println(count)
```

```
3
```

## Once

```go
func (s Seq[T]) Once() Seq[T]
```

Once returns a sequence that panics if iterated more than once — a development guard for the single-pass contract, not a synchronization mechanism. This is the one operator whose state deliberately lives outside the iteration closure. (catena.Once1 is unrelated: it constructs a one-element sequence.).

```go
// A development guard for the single-pass contract: the second
// consumption panics instead of silently re-running the producer.
s := catena.Of(1, 2).Once()
fmt.Println(s.Collect())

defer func() { fmt.Println("recovered:", recover()) }()
s.Collect()
```

```
[1 2]
recovered: catena: Once: sequence consumed more than once
```

## UntilDone

```go
func (s Seq[T]) UntilDone(ctx context.Context) Try[T]
```

UntilDone passes elements through until ctx is done, then yields (zero, ctx.Err()) and stops.

```go
// Cancellation enters at the edge rather than threading a context
// through every stage. The context's error arrives as an element.
ctx, cancel := context.WithCancel(context.Background())
cancel()

_, err := catena.Of(1, 2, 3).UntilDone(ctx).Collect()
fmt.Println(err)
```

```
context canceled
```
