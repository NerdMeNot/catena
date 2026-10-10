# Concurrency

The operators that start goroutines, in the `catena/parallel` package — the core package never does. Each is a stage that sits in a chain through `Seq.Then`, and each joins every goroutine it started before its iteration returns: on a normal end, an early break, a cancelled context and a panic alike. `Map`, `MapUnordered` and `ForEach` run a callback concurrently while the source stays on the consumer's goroutine; `Buffer`, `ChunkedWithin` and `Merge` run the source itself on a goroutine of its own. The [concurrency guide](../06-concurrency.md) explains the two models.

## Map

```go
func Map[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U]
```

Map returns a stage that yields f applied to each element, running up to n calls at once and yielding results in source order. A failed call yields (zero, err) and the stage continues. When ctx is done the stage stops consuming, yields (zero, ctx.Err()) once in place of any results it still held, and ends; over an empty source it yields nothing. f receives a context that is also cancelled the moment the consumer stops. The source runs on the consumer's goroutine. Panics if n <= 0.

:::caution
Starts up to n goroutines per iteration, all joined before it returns
:::

:::caution
Consumes at most n elements beyond those yielded, and holds their results: one slow call at the head delays everything behind it (MapUnordered does not)
:::

:::caution
A finished result is delivered, and a done ctx is noticed, only when the source next yields or ends: behind a slow live source both wait for the next element
:::

```go
// Eight lookups at once, results in the order the names went in.
lookup := func(_ context.Context, name string) (int, error) {
	return len(name), nil
}
lengths, err := catena.Of("ada", "grace", "linus").
	Then(parallel.Map(context.Background(), 8, lookup)).
	Collect()
fmt.Println(lengths, err)
```

```
[3 5 5] <nil>
```

## MapUnordered

```go
func MapUnordered[T, U any](ctx context.Context, n int, f func(context.Context, T) (U, error)) func(catena.Seq[T]) catena.Try[U]
```

MapUnordered is Map yielding each result as soon as its call finishes rather than in source order: the same elements, bounds and error rules, with no call able to delay another. Panics if n <= 0.

:::caution
Starts up to n goroutines per iteration, all joined before it returns
:::

:::caution
Consumes at most n elements beyond those yielded
:::

:::caution
A finished result is delivered only when the source next yields or ends
:::

```go
// Results arrive as their calls finish, so the order is not the
// input's; sort when it matters.
upper := func(_ context.Context, s string) (string, error) {
	return strings.ToUpper(s), nil
}
got, _ := catena.Of("b", "c", "a").
	Then(parallel.MapUnordered(context.Background(), 3, upper)).
	Collect()
slices.Sort(got)
fmt.Println(got)
```

```
[A B C]
```

## ForEach

```go
func ForEach[T any](ctx context.Context, n int, f func(context.Context, T) error) func(catena.Seq[T]) error
```

ForEach returns a terminal stage that calls f for each element, up to n calls at once, and returns the first error to occur — a call's own, or ctx.Err() — or nil once every call has succeeded. "First" is first in time, not in source order. A done ctx is an error even if every call that ran succeeded. On an error the remaining calls are cancelled through their context and joined before it returns. Panics if n <= 0.

:::caution
Starts up to n goroutines, all joined before it returns
:::

:::caution
Full drain on success
:::

```go
// A concurrent loop that returns the first error, having cancelled
// and waited for the rest.
var total atomic.Int64
add := func(_ context.Context, n int) error {
	total.Add(int64(n))
	return nil
}
err := catena.Range(1, 101, 1).Then(parallel.ForEach(context.Background(), 8, add))
fmt.Println(total.Load(), err)
```

```
5050 <nil>
```

## Buffer

```go
func Buffer[T any](n int) func(catena.Seq[T]) catena.Seq[T]
```

Buffer returns a stage that runs its source ahead of the consumer, on a goroutine of its own, holding up to n elements ready: a slow source and a slow consumer overlap instead of taking turns. Order is preserved. T cannot be inferred — write parallel.Buffer[int](64). Panics if n <= 0.

:::caution
Starts one goroutine per iteration, joined before it returns, so stopping waits for the source's current step
:::

:::caution
Buffers n elements and consumes up to n+1 beyond those yielded
:::

:::caution
Everything upstream runs concurrently with the consumer's loop body
:::

```go
// The source runs up to 16 elements ahead, on its own goroutine, so
// a slow producer and a slow consumer overlap. Order is unchanged.
squares := catena.Range(1, 6, 1).Map(func(n int) int { return n * n })
fmt.Println(squares.Then(parallel.Buffer[int](16)).Collect())
```

```
[1 4 9 16 25]
```

## ChunkedWithin

```go
func ChunkedWithin[T any](n int, d time.Duration) func(catena.Seq[T]) catena.Seq[[]T]
```

ChunkedWithin returns a stage that batches by size or by time, whichever comes first: it yields a chunk when it holds n elements, or when d has passed since the chunk's first element arrived, and yields the last partial chunk when the source ends. It never yields an empty chunk, so a silent source yields nothing. Every chunk is a fresh slice. T cannot be inferred — write parallel.ChunkedWithin[int](100, time.Second). Panics if n <= 0 or d <= 0.

:::caution
Starts one goroutine per iteration, joined before it returns, so stopping waits for the source's current step
:::

:::caution
Holds up to n elements in the chunk and runs the source up to another n ahead (64 at most), plus one
:::

:::caution
Everything upstream runs concurrently with the consumer's loop body
:::

```go
// Batches of up to three, or whatever has arrived within a minute.
// This source never pauses, so every chunk closes on size.
batches := catena.Range(1, 8, 1).
	Then(parallel.ChunkedWithin[int](3, time.Minute)).
	Collect()
fmt.Println(batches)
```

```
[[1 2 3] [4 5 6] [7]]
```

## Merge

```go
func Merge[T any](seqs ...catena.Seq[T]) catena.Seq[T]
```

Merge yields the elements of every sequence as they arrive, each sequence running on a goroutine of its own. Elements of one sequence keep their order; the interleaving between sequences is whatever the timing makes it. It ends when all have ended.

:::caution
Starts one goroutine per sequence, plus one, per iteration, all joined before it returns, so stopping waits for every source's current step
:::

:::caution
Buffers up to 64 elements between the sources and the consumer, so together they run that far ahead, plus one each
:::

:::caution
The sources run concurrently with each other and with the consumer's loop body
:::

```go
// Each sequence runs on its own goroutine; elements arrive as they
// are produced, so only the order within one sequence is fixed.
got := parallel.Merge(catena.Of(1, 2, 3), catena.Of(10, 20), catena.Of(100)).Collect()
slices.Sort(got)
fmt.Println(got)
```

```
[1 2 3 10 20 100]
```
