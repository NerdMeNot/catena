# Concepts

catena is small in concept count: four types, a naming law, and a handful
of package-wide rules that hold everywhere. Once you know these, every
operator's behavior is predictable from its name and doc comment.

## The four types

| Type | What it is | Evaluation |
|---|---|---|
| `Seq[T]` | `iter.Seq[T]` with 84 methods — 37 chainable, 47 terminal | lazy, possibly infinite |
| `Seq2[K, V]` | the stdlib pair currency; a *bridge* back to `Seq` | lazy |
| `Try[T]` | `iter.Seq2[T, error]` — fallible elements | lazy |
| `List[T]` | `[]T` with the mirrored operation set | eager |

`Seq` is a defined function type, not a struct, because `for v := range s`
has to work without ceremony. The price is that a `Seq` cannot carry a
size hint; the payoff is that conversion to and from `iter.Seq` is free in
both directions, and any third-party iterator adapts through `From` with
no copying.

`Seq2` is deliberately thin. It has what you need to get back to `Seq`
(`Keys`, `Values`, `MapTo`) and little else — it is a bridge to the
standard library's `maps.All`/`slices.All` world, not a second home.

## Eager vs lazy — the actual difference

The words get used loosely, so here is what they mean concretely. Take
the same two-stage computation both ways:

```go
// LAZY: Seq. These two lines run NOTHING — no filtering, no mapping.
// They build a description of work.
pending := catena.FromSlice(orders).
    Filter(isPaid).
    Map(invoiceFor)

invoices := pending.Take(5).Collect()
```

```go
// EAGER: List. Each line runs to completion before the next:
// Filter produces a finished []Order, Map a finished []Invoice.
invoices := catena.List[Order](orders).
    Filter(isPaid).
    Map(invoiceFor)
```

Three observable consequences of the lazy version:

- **Work happens at consumption, element by element.** Nothing runs until
  `Collect` (or `range`, or any terminal) pulls; then each order flows
  through *both* stages before the next order starts. There is no
  intermediate collection anywhere.
- **Early termination reaches backwards.** `Take(5)` means five orders
  are filtered and five invoices built — even if there are a million
  orders. The eager version would have built them all. This is also why
  lazy handles infinite sequences: `Take(1)` over `Generate(...)` returns.
- **The pipeline is a value describing work, not the work's result.**
  `pending` can be built in one place and consumed in another — which is
  also the source of laziness's one trap: consuming it twice runs the
  producer twice (see the single-pass contract below).

The eager version's consequences are the mirror image: every stage
allocates a full slice, nothing short-circuits — and in exchange, each
intermediate result is a plain, finished, inspectable `[]T`.

You can watch the difference happen:
[examples/01-basics](../examples/01-basics/main.go) runs the same two
stages both ways and prints the evaluation order — interleaved
`A,B,A,B,…` for lazy, staged `A,A,A…` then `B,B,B…` for eager.

### When eager is the right choice

Lazy is the headline feature, but `List` exists because eager genuinely
wins in common situations:

- **The data is small and already in memory.** For a config list of forty
  entries, streaming machinery buys nothing; `List.Map` preallocates
  exactly and is measurably faster than build-a-pipeline-then-collect.
- **You touch the result more than once.** A `List` is multi-pass by
  nature: index it, `range` it twice, hand it to two functions. Doing
  that with a `Seq` means either re-running the producer or thinking
  about whether re-running is even safe.
- **You want random access.** `l.At(3)`, `l.Len()`, `l.Last()` are O(1);
  the lazy equivalents are drains.
- **Debugging.** Each eager stage is a finished value you can print or
  put a breakpoint after. A lazy chain evaluates interleaved, inside the
  terminal.

And when lazy earns its keep: input that is large, unbounded, or behind
I/O; pipelines that can stop early (`Take`, `Find`, `Any`); and anywhere
an intermediate collection per stage would hurt — which is also what the
memory-class operators (`FoldBy`, `TopNBy`, `DedupeBy`) are for.

Crossing is always explicit — `l.AsSeq()` to go lazy (free: it's a view),
`s.ToList()` to materialize (a drain) — and an operator's evaluation
strategy is visible in its return type. Four `List` mirrors do hand back a
lazy value, because their `Seq` counterparts do: `WithIndex` and
`ZipWithNext` return a `Seq2`, `MapErr` and `FilterErr` return a `Try`.
Everything else returns a `List`. The two surfaces are conformance-checked
to agree:
`l.Op(x)` always equals `l.AsSeq().Op(x)` collected, so switching modes
never changes results, only when and how the work happens.

## Early termination is load-bearing

Building a pipeline runs nothing. Consuming it (a terminal, or `range`)
runs everything, once, streaming. `Take(1)` over an infinite sequence
returns after one element, through any number of stages — every operator
propagates early termination immediately, and the conformance suite proves
it per operator against an infinite source.

## Single-pass by contract

A `Seq` is a function; calling it twice runs the producer twice. Whether
that *works* depends entirely on the producer: `FromSlice` re-reads its
slice happily, `FromChan` yields nothing the second time, a database
producer would re-query or fail. The contract is therefore: **treat every
`Seq` as single-pass**, and treat re-iterability as a property of the
producer you happen to have (each constructor documents which it is).

**Operators do not affect this either way.** Every operator builds its state
inside the closure it returns, never in the enclosing call, so a chain is
re-iterable exactly when its root producer is —
`FromSlice(xs).Filter(f).Map(g).Chunked(3)` is as re-iterable as
`FromSlice(xs)`, no matter how long it grows. The conformance suite
re-iterates every operator to prove it. `Once()` is the sole deliberate
exception, and is the one operator whose state lives outside the closure.

So the question is never "is this pipeline safe to iterate twice?" but
"what constructor started it?" — a single lookup, answered in the table on
the `Seq` type.

`Once()` is the development guard — it panics on a second consumption.
Two operators deserve special awareness: `IsEmpty` consumes one element to
answer (lost forever on a single-pass source), and calling `Keys()` then
`Values()` on the same single-pass `Seq2` is a double consume — `Unzip`
is the one-pass spelling.

## Nil is empty

A nil `Seq`, `Seq2`, `Try`, or `List` behaves as the empty sequence in
every method and package function. `Pull` on nil returns an exhausted
iterator; `ToChan` on nil returns a closed channel. This removes a whole
class of nil checks from calling code.

## Panics are for misconstruction, and nothing else

Invalid *arguments* — a negative count, `Chunked(0)`, `Range` with a zero
step — panic at operator construction with a `catena:`-prefixed message,
so the stack trace points at the mistake. Zero counts are meaningful
values (`Take(0)` is empty, `Drop(0)` is identity), never panics.

Two operators panic on purpose while iterating, and both are named for it:
`Once` when a single-pass sequence is consumed twice, and `Must` with your
error value on the first error. Beyond those three cases the library never
panics, and it never recovers: a panic in your callback propagates
untouched, because a pipeline is not a fault boundary.
The one documented soft spot is Go's own: `comparable` admits interface
types, so `Distinct` over `Seq[any]` holding a slice panics at runtime —
the type system cannot catch it, and every `comparable`-constrained
function says so.

## Memory and drain classes

Every operator declares two things in its doc comment:

- **Memory**: streaming O(1), bounded by an argument (`TakeLast(n)`),
  bounded by distinct keys (`Distinct`, `FoldBy`), or the whole input
  (`Sorted*`, `Reversed`).
- **Drain**: safe on infinite input (`First`, `Take`), conditional
  (`Any`, `Find` — terminate only if the answer arrives), or full-drain
  (`Count`, `Last`, `Collect` — these hang on infinite input, and are
  marked ⚠).

## Ordering, ties, and collisions

Stated once, applied everywhere: encounter order is preserved by
everything that doesn't sort; map-building terminals are last-wins on key
collisions (like `maps.Collect`); `Distinct` and `Max`/`Min` ties are
first-wins; sorts are stable, including `TopNBy` through its bounded heap;
NaN orders below everything (`cmp.Compare`), so aggregates always agree
with sorts.

## The naming law

A suffix means the same thing on every operator:

| Suffix | Meaning |
|---|---|
| `-By` | a selector produces the key that drives the operation |
| `-Of` | a selector produces a projection; projections come out |
| `-With` | takes a comparator or equality function |
| `-Indexed` | the callback also receives the element index |
| `-Err` | the callback may fail; the result is a `Try` |

So `MaxBy(sel)` returns the *element* with the largest key and `MaxOf(sel)`
returns the *key* — and having learned that once, `MinBy`/`SumOf`/
`SortedBy` need no lookup.

Operations that constrain the element type (`Distinct`, `Sorted`, `Sum`,
`Max`, …) are package functions rather than methods — a method on
`Seq[T any]` may require nothing of `T`. The batch operators (`Chunked`,
`ChunkedBy`, `Windowed`) are package functions for a stranger reason: a
method on `Seq[T]` returning `Seq[[]T]` is an instantiation cycle the
compiler rejects. The chain continues normally after either kind.
