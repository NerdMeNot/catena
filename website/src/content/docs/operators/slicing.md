---
title: "Slicing"
description: "Taking a window of the sequence by position or by a predicate. Take and TakeWhile consume only what they emit, so either one bounds an infinite source; TakeLast and DropLast cannot, since they have to reach the end first."
sidebar:
  order: 3
---

Taking a window of the sequence by position or by a predicate. Take and TakeWhile consume only what they emit, so either one bounds an infinite source; TakeLast and DropLast cannot, since they have to reach the end first.

## Take

```go
func (s Seq[T]) Take(n int) Seq[T]
```

Take yields at most the first n elements, consuming exactly as many as it yields. Panics if n is negative.

```go
// The next three invoice numbers from a counter that never ends.
// Take consumes exactly what it emits, so it bounds an infinite
// source.
invoices := catena.Generate(1041, func(n int) int { return n + 1 }).
	Map(func(n int) string { return fmt.Sprintf("INV-%d", n) })

fmt.Println(invoices.Take(3).Collect())
```

```
[INV-1041 INV-1042 INV-1043]
```

## TakeWhile

```go
func (s Seq[T]) TakeWhile(pred func(T) bool) Seq[T]
```

TakeWhile yields elements until pred first returns false.

```go
// The build steps that passed before the first failure. It stops at
// the first element that fails — unlike Filter, which would keep
// testing the rest and let "package" through.
type step struct {
	Name   string
	Passed bool
}
steps := catena.Of(
	step{"lint", true}, step{"build", true},
	step{"test", false}, step{"package", true},
)

fmt.Println(steps.
	TakeWhile(func(s step) bool { return s.Passed }).
	Map(func(s step) string { return s.Name }).
	Collect())
```

```
[lint build]
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
// tail -n 2: the end of a log. It buffers only n lines, but has to
// read the source to its end to know which they are.
log := catena.Of(
	"12:00:01 starting",
	"12:00:02 listening on :8080",
	"12:04:37 upstream timeout",
	"12:04:38 shutting down",
)

log.TakeLast(2).ForEach(func(line string) { fmt.Println(line) })
```

```
12:04:37 upstream timeout
12:04:38 shutting down
```

## Drop

```go
func (s Seq[T]) Drop(n int) Seq[T]
```

Drop skips the first n elements. Panics if n is negative.

```go
// Pagination: skip the pages already shown, then take one page.
results := catena.Of("ada", "bob", "cyd", "dee", "eve", "fay", "gus")
page, size := 1, 3

fmt.Println(results.Drop(page * size).Take(size).Collect())
```

```
[dee eve fay]
```

## DropWhile

```go
func (s Seq[T]) DropWhile(pred func(T) bool) Seq[T]
```

DropWhile skips elements until pred first returns false, then yields the rest.

```go
// Skips the comment banner at the top of a config file. Only the
// leading run is dropped; once the predicate fails, everything after
// is kept — including the later comment.
config := catena.Of(
	"# managed by ops",
	"# do not edit",
	"port = 8080",
	"# seconds",
	"timeout = 30",
)

fmt.Println(config.
	DropWhile(func(line string) bool { return strings.HasPrefix(line, "#") }).
	JoinToString("; ", catena.Self[string]))
```

```
port = 8080; # seconds; timeout = 30
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
// Strips a totals row off the end of an export without knowing how
// long the export is. It runs n elements behind the source, and
// buffers only those.
export := catena.Of("ada,36.00", "bob,41.50", "eve,12.25", "TOTAL,89.75")

fmt.Println(export.DropLast(1).Collect())
```

```
[ada,36.00 bob,41.50 eve,12.25]
```

## Step

```go
func (s Seq[T]) Step(n int) Seq[T]
```

Step yields the first element and every nth element after it. Panics if n <= 0.

```go
// Downsampling: one CPU reading in every three is enough for the
// chart. The first element always survives, then every nth after it.
cpuPercent := catena.Of(12, 15, 11, 40, 42, 38, 90, 88)

fmt.Println(cpuPercent.Step(3).Collect())
```

```
[12 40 90]
```
