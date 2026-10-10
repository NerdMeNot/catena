---
title: "Concurrency"
description: "The operators that start goroutines, in the catena/parallel package — the core package never does. Each is a stage that sits in a chain through Seq.Then, and each joins every goroutine it started before its iteration returns: on a normal end, an early break, a cancelled context and a panic alike. Map, MapUnordered and ForEach run a callback concurrently while the source stays on the consumer's goroutine; Buffer, ChunkedWithin and Merge run the source itself on a goroutine of its own. The concurrency guide explains the two models."
sidebar:
  order: 15
---

The operators that start goroutines, in the `catena/parallel` package — the core package never does. Each is a stage that sits in a chain through `Seq.Then`, and each joins every goroutine it started before its iteration returns: on a normal end, an early break, a cancelled context and a panic alike. `Map`, `MapUnordered` and `ForEach` run a callback concurrently while the source stays on the consumer's goroutine; `Buffer`, `ChunkedWithin` and `Merge` run the source itself on a goroutine of its own. The [concurrency guide](/guides/concurrency/) explains the two models.

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
// A product page needs four products, and each is a request. Eight at
// a time, and they come back in the order they were asked for —
// whichever response happened to arrive first.
ctx := context.Background()
ids := catena.Of(101, 102, 103, 104)

products, err := ids.Then(parallel.Map(ctx, 8, getProduct)).Collect()
if err != nil {
	fmt.Println("page failed:", err)
	return
}
for _, p := range products {
	fmt.Printf("%d %-9s $%d\n", p.ID, p.Name, p.Price)
}
```

```
101 keyboard  $89
102 mouse     $35
103 monitor   $329
104 webcam    $59
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
// Ask three mirrors for the same file and take whichever answers
// first. Results arrive as their calls finish, so First returns as
// soon as one does — and stopping cancels the other two requests and
// waits for them to end before First returns.
ctx := context.Background()
mirrors := catena.Of("eu-slow", "us-fast", "ap-slow")

winner, ok := mirrors.
	Then(parallel.MapUnordered(ctx, 3, download)).
	Ignore().
	First()
fmt.Println(winner, ok)
```

```
2048 bytes from us-fast true
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
// A price changed: purge it from every edge cache, four nodes at a
// time. ForEach returns the first error, having cancelled and waited
// for the purges still running — or nil once every node has confirmed.
ctx := context.Background()
nodes := catena.Of("fra1", "iad2", "sin1", "syd1", "gru1", "nrt1")

var confirmed atomic.Int64
err := nodes.Then(parallel.ForEach(ctx, 4, func(ctx context.Context, node string) error {
	if err := purgeCache(ctx, node); err != nil {
		return fmt.Errorf("%s: %w", node, err)
	}
	confirmed.Add(1)
	return nil
}))
fmt.Println("purged on", confirmed.Load(), "nodes, err:", err)
```

```
purged on 6 nodes, err: <nil>
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
// Exporting orders: reading them is slow and so is uploading them.
// Behind a buffer the reader runs ahead on its own goroutine while
// the upload is in flight, so the two waits overlap instead of
// adding up. The order of the rows is unchanged.
uploaded := 0
for row := range exportedOrders().Then(parallel.Buffer[string](64)) {
	upload(row)
	uploaded++
}
fmt.Println(uploaded, "rows uploaded")
```

```
5 rows uploaded
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
// Click events arrive on a channel and go to the database in bulk:
// up to three rows per INSERT, but never held back for more than
// 50ms waiting for a batch to fill. Chunked alone would sit on the
// last partial batch until more events came.
ctx := context.Background()
events := catena.FromChan(ctx, clicks())

for batch := range events.Then(parallel.ChunkedWithin[string](3, 50*time.Millisecond)) {
	fmt.Println("INSERT", len(batch), "rows:", batch)
}
```

```
INSERT 3 rows: [home cart pay]
INSERT 3 rows: [home search item]
INSERT 1 rows: [cart]
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
// One stream out of three services' logs, each read on its own
// goroutine. Lines from one service stay in order; how the services
// interleave is down to timing, so this counts rather than prints.
logs := parallel.Merge(serviceLog("api"), serviceLog("worker"), serviceLog("db"))

errors := logs.
	Filter(func(line string) bool { return strings.Contains(line, "ERROR") }).
	TallyBy(func(line string) string { return strings.Fields(line)[0] })
fmt.Println("api:", errors["api"], "worker:", errors["worker"], "db:", errors["db"])
```

```
api: 2 worker: 0 db: 1
```
