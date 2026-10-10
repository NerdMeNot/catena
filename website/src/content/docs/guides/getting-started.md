---
title: "Getting started"
description: "Five minutes, four ideas: a pipeline, a grouped total, errors that the caller gets to decide about, and slow calls run several at a time. By the end you will know whether catena fits the code you write."
sidebar:
  order: 1
---

Five minutes, four ideas: a pipeline, a grouped total, errors that the
caller gets to decide about, and slow calls run several at a time. By
the end you will know whether catena fits the code you write.

It needs Go 1.27 or later — the chainable API is built on generic
methods, which arrived in 1.27 — and has no dependencies.

```sh
go get github.com/NerdMeNot/catena
```

## A first pipeline

```go
import "github.com/NerdMeNot/catena"

// The first five accounts with a failed login today.
suspects := catena.FromSlice(events).
    Filter(func(e Event) bool { return e.Kind == "login" && !e.OK }).
    Map(func(e Event) string { return e.Account }).
    Take(5).
    Collect()
```

Read it top to bottom and it says what it does. Nothing runs until
`Collect` asks for the result, and `Take(5)` means the scan stops at the
fifth match — a day with ten million events is read only as far as it
needs to be. The loop this replaces has a counter, a `break`, and an
`append`; none of them were the point.

A callback does not have to be a closure. If `Event` has a method
`FailedLogin() bool`, the method expression `Event.FailedLogin` is
already a function of an event, and `Filter(Event.FailedLogin)` reads
better still.

`Seq` is Go's `iter.Seq` underneath, so `range` works directly and nothing
needs converting at either edge:

```go
for v := range catena.FromSlice(xs).Filter(pred) {
    ...
}
s := catena.From(maps.Keys(m))   // any iter.Seq adapts for free
std := s.Seq()                   // and converts back for free
```

## Grouped aggregation without the intermediate map

The operator to learn first is `FoldBy`: it folds each element into a
per-key accumulator as elements stream past. `GroupBy` retains every
element; `FoldBy` retains one accumulator per distinct key.

```go
totals := catena.FromSlice(orders).FoldBy(
    func(o Order) string { return o.Customer },    // key
    func(string) int { return 0 },                 // initial accumulator
    func(sum int, o Order) int { return sum + o.Amount },
)
```

Its companion is `TopNBy`, which keeps a bounded heap of n instead of
sorting everything:

```go
top := catena.FromSlice(orders).TopNBy(10, func(o Order) int { return o.Amount })
```

On a large scan, these two are the difference between a working pipeline
and an out-of-memory kill — measured numbers in
[05-performance.md](/guides/performance/).

## Errors, when the data can fail

Parsing, scanning, and I/O produce a `Try[T]` — a sequence where each
element either succeeded or carries an error. The pipeline doesn't decide
what an error means; the consumer does:

```go
nums := catena.FromSlice(lines).MapErr(strconv.Atoi)

vals, err := nums.Collect()    // stop at the first bad line
vals, errs := nums.CollectAll() // keep everything, gather all errors
vals := nums.Ignore().Collect() // skip bad lines, keep going
```

To stop at the first error and keep chaining, `UntilErr` is `Collect`'s
policy as a sequence — check the error once the pipeline has run:

```go
var err error
total := nums.UntilErr(&err).Then(catena.Sum)
```

The full story — including the shipped producers for files, paginated
APIs and `sql.Rows`, and how to write your own — is in
[03-error-handling.md](/guides/error-handling/).

## When each element is slow

A callback that waits — on a request, a query, a file — can run several
at a time without the pipeline changing shape. Concurrency is a separate
import, `catena/parallel`, and a stage goes into the chain with `Then`:

```go
bodies, err := catena.FromSlice(urls).
    Filter(isHTTPS).
    Then(parallel.Map(ctx, 8, fetch)). // eight at a time, results in order
    Collect()
```

A failed fetch is an element like any other, so the three policies above
still apply, and every goroutine has exited by the time `Collect`
returns. [06-concurrency.md](/guides/concurrency/) has the rest.

## Eager when you want it

`List[T]` is a `[]T` with the same operation set, evaluated eagerly with
exact preallocation. Use `Seq` for I/O, large scans, and anything
short-circuiting; use `List` for small collections you touch more than
once. Crossing between them is always explicit: `l.AsSeq()` and
`s.ToList()`. If the eager/lazy distinction is new, the concepts doc
[walks through what actually runs when, and when eager is the better
choice](/guides/concepts/#eager-vs-lazy--the-actual-difference).

## Four rules to know before anything bites

1. **Treat every `Seq` as single-pass.** Whether re-iteration works depends
   on the producer; slices and ranges are safe, channels and rows are not.
   `Once()` turns a double consume into a loud panic during development.
2. **Nil is empty.** A nil `Seq` behaves as an empty sequence everywhere —
   no nil checks needed at call sites.
3. **Bad arguments panic at construction**, with a `catena:` prefix, at the
   line that made the mistake — not three files later inside a range loop.
   Zero counts are valid values, never panics.
4. **Operations that constrain the element type are package functions.**
   `catena.Distinct(s)`, not `s.Distinct()` — a method on `Seq[T any]` may
   require nothing of `T`, so anything needing `comparable`, `cmp.Ordered`
   or a numeric `T` lives at package level. The chain continues normally
   afterwards — `catena.Distinct(s).Filter(f)` — or stays unbroken with
   `Then`: `s.Then(catena.Distinct).Filter(f)`. If the compiler tells you
   `Seq[int] has no field or method Distinct`, this is why — the operator
   exists, it is just spelled differently.

The rest of the contract system is in [02-concepts.md](/guides/concepts/).

## If you are coming from somewhere else

Most names are the Kotlin ones. Where your habit differs:

| You'd type | catena |
|---|---|
| `Where` (LINQ) | `Filter` |
| `Select` (LINQ) | `Map` |
| `SelectMany` (LINQ) | `FlatMap` |
| `Skip` (LINQ) | `Drop` |
| `OrderBy` / `OrderByDescending` | `SortedBy` / `SortedByDesc` |
| `First(pred)` (LINQ) | `Find` — `First()` takes no predicate |
| `Count(pred)` (LINQ) | `CountWhere` — `Count()` takes no predicate |
| `ToDictionary` / `associateBy` | `Associate` / `IndexBy` |
| `Aggregate` (LINQ) / `fold` | `Fold` |
| `Distinct()` (method, anywhere) | `catena.Distinct(s)` — a package function |
| `chunked` / `chunks` | `catena.Chunked(s, n)` — a package function |
| `sortedByDescending().take(n)` | `TopNBy(n, sel)` — bounded memory |
| `peek` / `inspect` | `OnEach` |
| `any` / `all` / `none` | `Any` / `All` / `None` |

`Filter` and `Map` keep their Kotlin/Rust names rather than LINQ's because
they match `slices.DeleteFunc` and the rest of Go's own vocabulary.

The full lookup — from `lo`, Kotlin and LINQ — is in
[08-coming-from.md](/guides/coming-from/).
