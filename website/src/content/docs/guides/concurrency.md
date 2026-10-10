---
title: "Concurrency"
description: "You have a function that waits — on an API, a query, a disk — and a few hundred things to call it for. One at a time, the program spends its life waiting. catena/parallel calls it for several at once:"
sidebar:
  order: 6
---

You have a function that waits — on an API, a query, a disk — and a few
hundred things to call it for. One at a time, the program spends its life
waiting. `catena/parallel` calls it for several at once:

```go
import "github.com/NerdMeNot/catena/parallel"

// func loadCustomer(ctx context.Context, id CustomerID) (Customer, error)

customers, err := catena.FromSlice(ids).
    Then(parallel.Map(ctx, 8, loadCustomer)). // eight calls in flight, results in order
    Collect()
```

`loadCustomer` is an ordinary function: it takes a context, returns a
value or an error, and knows nothing about being run concurrently. What
you get back is what you would have got from a loop — the customers, in
the order of the ids, or the first error — in an eighth of the time.

What you did not write is the part that goes wrong by hand: the
semaphore, the `WaitGroup`, the results slice indexed so they come back
in order, the error channel, cancelling the rest when one fails, and
making sure nothing is still running when you return.

The result is a `Try[Customer]`, so everything in
[error handling](/guides/error-handling/) applies unchanged: `Collect` stops
at the first failure and cancels the calls still in flight, `CollectAll`
runs them all and gathers the failures, `Ignore` skips them.

The core package never starts a goroutine on your behalf — `ToChan` is
the one exception, and it says so. Everything that does lives here, so
that importing `parallel` is the moment you opt in.

## What people use it for

If a loop spends its time waiting, or has more cores than it uses, one of
these is probably it. Each links to a complete, runnable example.

| You want to | Reach for |
|---|---|
| Call an API for every item — enrich orders with their customer, resolve a list of ids | [`Map`](/operators/concurrency/#map) |
| Check a list of links, hosts or certificates, and keep going past the ones that fail | [`Map` with `CollectAll`](/operators/concurrency/#keep-going-past-failures) |
| Hash, resize, transcode or parse every file, on every core | [`Map`](/operators/concurrency/#cpu-work) |
| Ask several replicas or mirrors and use the first to answer | [`MapUnordered` with `First`](/operators/concurrency/#mapunordered) |
| Send webhooks, notifications or cache purges, and stop at the first failure | [`ForEach`](/operators/concurrency/#foreach) |
| Load the three unrelated things a page needs, at once | [`ForEach` over functions](/operators/concurrency/#different-calls) |
| Bulk-insert a live stream without holding rows back for a full batch | [`ChunkedWithin`](/operators/concurrency/#chunkedwithin) |
| Keep reading while the consumer is busy — decode while uploading, parse while compressing | [`Buffer`](/operators/concurrency/#buffer) |
| Combine several feeds — shards, services, partitions — into one stream | [`Merge`](/operators/concurrency/#merge) |
| Walk a paginated API and fetch the details of each item concurrently | `source.Paginate`, `UntilErr`, then `Map` — [below](#errors-and-cancellation) |

## When it is worth it

A stage hands every element to a pool of goroutines. With a callback that
does nothing that costs about 0.26µs an element — less than a hand-written
loop with a semaphore, and some two hundred times what a plain `Map`
costs. So the
rule is the usual one: use it for callbacks that *wait* — a request, a
query, a file — where eight at a time is eight times the throughput, and
not for callbacks that compute something small, where the goroutine costs
more than the work. The numbers are in the
[performance guide](/guides/performance/).

## Stages and `Then`

A package cannot add methods to another package's type, so `parallel` cannot
give `Seq` a `ParMap` method. Its operators are *stages* instead:
`parallel.Map(ctx, 8, fetch)` returns a function from a sequence to its result,
and `Seq.Then` applies a function to the sequence it is called on. The
chain reads left to right, and the types are inferred.

`Then` is not special to `parallel`. Any function of one sequence fits, which
includes every one-argument package function in the core:

```go
catena.FromSlice(words).
    Then(catena.Distinct).
    Then(catena.Sorted).
    Take(10)
```

## The guarantee

**Every goroutine a stage starts has exited before its iteration
returns.** On a normal end, on an early `break`, when the context is
cancelled, and when something panics. There is no "remember to drain", no
goroutine left blocked on a channel nobody reads, and nothing to leak:
when `Collect` returns or your `for` loop exits, the work is over.

That is structured concurrency, and it has one cost you should know
before you rely on it: **stopping waits.** A stage that must join its
goroutines cannot return while one of them is still running. So:

- A callback given to `Map`, `MapUnordered` or `ForEach` must honour its
  context. The context it receives is cancelled the moment the consumer
  stops; a callback that ignores it keeps the stage waiting until it
  finishes on its own.
- A source behind `Buffer`, `ChunkedWithin` or `Merge` must return from
  its current step. A source blocked forever in a read that nothing
  interrupts blocks the stage forever with it.

This is also why there is no `parallel.Timeout`. A timeout fires because the
source is stuck — and a stage that cannot abandon a stuck goroutine
cannot return to report it. Put the deadline where it can be honoured: in
the context your source or callback reads.

## Two execution models

Every operator is one of two kinds, and its doc comment says which.

| | `Map`, `MapUnordered`, `ForEach` | `Buffer`, `ChunkedWithin`, `Merge` |
|---|---|---|
| The source runs on | your goroutine | a goroutine of its own |
| What runs concurrently | only the callback | everything upstream of the stage |
| Elements taken ahead of what you have received | at most `n` | `n+1` for `Buffer`; up to 64 more for the others |
| The cost | a finished result waits for the source's next element | upstream code must be safe to run alongside your loop body |

**Caller-driven** stages invoke their source exactly as the core
operators do — directly, on the goroutine that is consuming. Nothing
upstream of a `parallel.Map` becomes concurrent: a `Filter` before it, a
database cursor feeding it, a closure that counts — all still run on one
goroutine, yours. Only the callback fans out.

The price is in how results come back. The stage hands you a result when
the source gives it an element (or ends), because that is when it has
control. Over a slice, a file or a query that is invisible. Over a live
source that goes quiet — a channel, a socket — a result that finished
during the silence is delivered when the next element arrives.

**Producer** stages are the other way round. The source runs on its own
goroutine and elements cross to yours over a channel, which is what lets
`ChunkedWithin` flush on a timer while the source is idle and lets
`Buffer` read ahead. The price is that everything upstream of the stage
now runs concurrently with your loop body. If a callback up there touches
state your loop also touches, that is a data race, exactly as if you had
written `go` yourself.

## Order

`Map` yields results in source order, whatever order the calls finish in.
It does that by holding results back: one slow call at the head of the
line delays everything behind it, and at most `n` results wait.

`MapUnordered` yields each result as its call finishes. Nothing waits on
anything else, and the output is some permutation of the input's. Use it
when order does not matter — it is never slower, and under uneven latency
it is much faster to first result.

`ForEach` is the terminal: it runs the callback for its effect and
returns the first error, having cancelled and joined the rest.

```go
err := catena.FromSlice(jobs).Then(parallel.ForEach(ctx, 16, run))
```

## A handful of different calls at once

Not every concurrent job is the same function over many inputs. A page
often needs three *different* things that have nothing to do with each
other, and there is no reason to fetch them in turn. A sequence of
functions does it:

```go
var user User
var orders []Order
var recs []Product

err := catena.Of(
    func(ctx context.Context) (err error) { user, err = loadUser(ctx, id); return },
    func(ctx context.Context) (err error) { orders, err = loadOrders(ctx, id); return },
    func(ctx context.Context) (err error) { recs, err = loadRecs(ctx, id); return },
).Then(parallel.ForEach(ctx, 3, func(ctx context.Context, load func(context.Context) error) error {
    return load(ctx)
}))
```

All three run together; the first to fail cancels the other two; and by
the time `err` is assigned all three have returned, so reading `user`,
`orders` and `recs` afterwards is safe.

## Errors and cancellation

A failed callback is an element, not an abort. The stage keeps going, and
the consumer decides:

```go
stage := catena.FromSlice(ids).Then(parallel.Map(ctx, 8, load))

vals, err  := stage.Collect()          // stop at the first failure; the rest are cancelled
vals, errs := stage.CollectAll()       // run everything, gather every failure
vals       := stage.Ignore().Collect() // run everything, keep the successes
```

When the parent context is done, the stage stops consuming, yields the
context's error once, and ends — the same shape as `UntilDone`.

A panic in a callback or in the source is re-raised on your goroutine,
with its original value, after everything has been joined.

If the input is itself a `Try` — rows from a query, pages from an API —
give it a policy before the stage, since the stages take a plain `Seq`.
`UntilErr` is usually the one: it stops at the source's first error and
holds it for you to check once the pipeline has run.

```go
var srcErr error
users, err := pages.UntilErr(&srcErr).
    Then(parallel.Map(ctx, 8, enrich)).
    Collect()
if err = errors.Join(srcErr, err); err != nil {
    return err
}
```

Reading `srcErr` afterwards is safe even where the source ran on another
goroutine: every stage has joined before `Collect` returns.

## Batching by time

`Chunked` batches by size, which is wrong for a live stream: the last
few elements sit in a half-full batch until more arrive. `ChunkedWithin`
closes a batch when it is full *or* when its oldest element has waited
long enough:

```go
for batch := range events.Then(parallel.ChunkedWithin[Event](500, time.Second)) {
    insert(batch) // up to 500 rows, never more than a second stale
}
```

The window opens when a batch receives its first element, not on a fixed
tick, and an idle source yields nothing — there are no empty batches.

`Buffer` and `ChunkedWithin` cannot infer their element type, since
nothing in their arguments mentions it: write `parallel.Buffer[Event](64)`.

## How this is tested

The same way the core is — by a conformance harness that every operator
is registered in, and a check that fails the build for one that is not.
The harness runs under `testing/synctest`, which gives it virtual time, so
its timing assertions are exact rather than tolerant: ten one-second calls
at `n = 3` take four seconds, not "about four".

The guarantee above is the part most worth distrusting, so it is tested
against callbacks that are deliberately slow to stop — each one outlives
its own cancellation by a second. A stage that cancelled without waiting
would return while they were still running, and the harness would see it
every time rather than once in a thousand runs.

## Reference

[Concurrency operators](/operators/concurrency/) — every signature,
its cautions, and a verified example.
