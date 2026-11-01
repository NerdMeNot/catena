---
title: "Performance"
description: "The promise is precise: any overhead catena carries beyond a hand-written loop belongs to Go's iterator protocol, not to the library — and that claim is measured, not asserted."
sidebar:
  order: 5
---

The promise is precise: **any overhead catena carries beyond a hand-written
loop belongs to Go's iterator protocol, not to the library** — and that
claim is measured, not asserted.

## The three-way baseline

Every hot shape is benchmarked three ways in `bench_baseline_test.go`:

- **Hand** — a plain loop, no iterators. The absolute floor.
- **Raw** — the same pipeline hand-built from bare `iter.Seq` closures.
  The floor of the rangefunc mechanism itself.
- **Catena** — the library.

The standing rule is *catena ≤ raw wherever the two compile on equal
terms*; an unexplained regression against the raw column is a library bug.
Two rows below break that rule by 4%, and the footnote is the
explanation — both are single-stage shapes where the raw baseline is
inlined into its consumer and a library call across a package boundary
cannot be. Current numbers (Apple M5 Max, Go 1.27, 100k elements):

| Shape | Hand | Raw closures | catena |
|---|---|---|---|
| 4-stage pipeline (filter→map→filter→sum) | 25µs | 353µs | **261µs** |
| `Sum` | 24µs | 178µs | **24µs** |
| `Contains` (absent) | — | 130µs | **24µs** |
| `Distinct` + count | — | 369µs | 382µs (+4%)† |
| single-stage `Map` | — | 167µs | 174µs (+4%)† |

The hand-written loops are measured as functions of their own, the way
they would exist in a program. Written inline in a benchmark body they
are compiled under the rule that nothing in the body may be optimised
away, which made them look up to four times slower than they are — and
made earlier versions of this table flatter the library.

† The raw baselines compile in the same package as their consumer, so the
inliner collapses them completely; catena crosses a package boundary.
In-package *generic* copies of the raw code run at raw speed, which pins
the residual on inlining context — not on generics, and not on the library.

## What the trade actually looks like

Numbers without the code are half the story. This is the 4-stage pipeline
from the table — the two versions the 25µs and 261µs belong to:

```go
// Hand loop: 25µs. One pass, no abstractions — and the stages are
// interleaved into control flow. Adding, removing, or reordering a step
// means re-deriving the continue/nesting structure by hand.
sum := 0
for _, v := range orders {
    if v%2 != 0 {
        continue
    }
    v *= 3
    if v > 100 {
        sum += v
    }
}
```

```go
// catena: 261µs. Each stage is one line, in pipeline order; inserting a
// stage is inserting a line. 2.4ns per element across the whole chain
// is what that structure costs.
sum := catena.FromSlice(orders).
    Filter(func(v int) bool { return v%2 == 0 }).
    Map(func(v int) int { return v * 3 }).
    Filter(func(v int) bool { return v > 100 }).
    SumOf(catena.Self[int])
```

On 100k trivial integer operations that difference is 223 microseconds,
total. If the loop body does anything real — parses a line, hits a map,
allocates — the per-element work dwarfs the protocol overhead and the two
versions converge. The honest advice cuts both ways: don't put catena in a
hot inner loop doing arithmetic on a small slice, and don't hand-unroll an
I/O pipeline to save nanoseconds that its reads will bury.

The comparison changes character once errors and state enter. The same
"parse, filter, aggregate per key" logic, both ways:

```go
// Hand: the error policy, the position counter, and the aggregation
// bookkeeping are woven through the loop — and each future reader
// re-derives which parts are policy and which are plumbing.
totals := make(map[string]int)
lineNo := 0
for sc.Scan() {
    lineNo++
    e, err := parseLine(sc.Text())
    if err != nil {
        return nil, fmt.Errorf("line %d: %w", lineNo, err)
    }
    if e.Status != 200 {
        continue
    }
    totals[e.Path] += e.Ms
}
```

```go
// catena: the same abort-on-first-error policy, as one Fold — and
// switching policy to "skip bad lines" is swapping in .Ignore(), not
// rewriting the loop.
totals, err := lines.
    MapErr(parseLine).
    WrapErr(lineContext).
    Filter(func(e Entry) bool { return e.Status == 200 }).
    Fold(map[string]int{}, func(m map[string]int, e Entry) map[string]int {
        m[e.Path] += e.Ms
        return m
    })
```

And for the fused operators, there is no trade at all — the catena version
is the *faster* one, because it changes the memory class. `TopNBy` against
the natural hand-written spelling:

```go
// Hand: sort a copy, take ten. O(n log n) time, O(n) memory — 803 KB
// and 3.3ms on the benchmark input.
sorted := slices.Clone(orders)
slices.SortFunc(sorted, func(a, b Order) int { return b.Amount - a.Amount })
top := sorted[:10]
```

```go
// catena: bounded heap of 10. 1 KB and 275µs — 12× faster than the
// hand-written sort above, and shorter.
top := catena.FromSlice(orders).TopNBy(10, func(o Order) int { return o.Amount })
```

Two comparisons live here and they answer different questions. Against a
hand-written clone-and-sort, `TopNBy` is 12× faster and holds 803 KB less
— that is the honest number if you would otherwise reach for `slices`.
Against the same idea spelled through catena's own sort,
`SortedDesc(s).Take(10)`, it is 32× faster and holds 1.7 MB less, because
that spelling pays for the pipeline as well as the sort. Both are
measured: `BenchmarkSortedTakeN_Hand` and `BenchmarkSortedTakeN`.

The bounded-heap loop *can* be written by hand, of course — it's ~60 lines
with sift-up/sift-down and stable tie-breaking, and now it's yours to
test. That is the actual shape of the bargain: for trivial loops you pay
nanoseconds for structure; for stateful ones you pay nothing and stop
maintaining the plumbing.

## Where catena's speed comes from

Two decisions about how an operator consumes its source.

**It calls the source; it does not range over it.** A `range` statement
inside an operator is lowered through the compiler's rangefunc state
machine, which costs real time per element per stage; a direct call chain
doesn't. This took the 4-stage pipeline from parity with raw closures to
25% faster.

**It calls it in a way the compiler can see through.** A nil sequence
must behave as an empty one, and the first implementation got that by
calling through an adapter that returned either the sequence or an empty
stand-in — which made every source an opaque function value. Operators
now make the call through a method small enough to inline, leaving a nil
check and a plain call at the call site. Where the caller built the
source in the same function — `catena.Sum(catena.FromSlice(xs))` — the
compiler now sees a known closure and flattens the whole thing into a
loop. That is why `Sum` and `Contains` run at the speed of the hand
loop: they are the hand loop, by the time the compiler is done. It took
`Sum` from 71µs to 24µs, and the grouping and join operators 10–16%.

It does not reach across several stages: a chain of four closures is
still four indirect calls per element, which is the next section.

Two more implementation choices with measured effects:

- `SortedBy` is decorate-sort-undecorate: the selector runs exactly once
  per element, not once per comparison (~29× fewer calls at n=10k).
- `TopNBy` keeps a bounded heap with index-tracked ties, so it is stable
  like the sorts without sorting.

## What the remaining 10× is

Against a hand loop on trivial in-memory arithmetic, a 4-stage catena
pipeline costs about 2.4ns more per element across its four stages. That is the price of the
iterator protocol: each stage boundary is an indirect closure call the
compiler cannot flatten across packages. It is the same price the raw
`iter.Seq` ecosystem pays — more, actually, as the table shows.

Spend it where it buys something: pipelines over I/O, unbounded input,
four-plus stages, or intermediate types that change enough that a loop
needs named temporaries. Don't spend it summing a small slice in a hot
inner loop — the README says this plainly and means it.

## Where fusion wins outright

| Benchmark | Result |
|---|---|
| `FoldBy` vs `GroupBy` + fold per bucket | same speed, **2.97 MB → 1.2 KB** allocated |
| `TopNBy(10)` vs a hand-written clone-sort-slice | **12× faster**, 803 KB → 1 KB |
| `TopNBy(10)` vs `SortedDesc().Take(10)` | **32× faster**, 1.7 MB → 1 KB |
| `List.Map` (exact prealloc) vs `Seq` map + collect | **6× faster**, 1 allocation |
| `Sample(10)` vs collect-and-shuffle | **4.4× faster**, 803 KB → 456 B |

These are memory-class differences, not constant factors: `FoldBy` is
bounded by distinct keys where `GroupBy` retains every element, and
`TopNBy` is bounded by n where a sort buffers the world.

## The operators added in 1.5

The same three columns for the grouping and join operators (100k
elements; the joins are 10k against 1k):

| Operator | Hand | Raw closures | catena |
|---|---|---|---|
| `FoldedBy` | 650µs | 706µs | **700µs** |
| `GroupedBy` | 954µs | 1044µs | **1038µs** |
| `GroupJoinBy` | 62µs | 79µs | **73µs** |
| `LeftJoinBy` | 155µs | 299µs | 326µs (+9%) |
| `IsSorted` | 23µs | 47µs | **44µs** |
| `Duplicates`, after a `Map` | 398µs | 386µs | 479µs (+24%)‡ |

‡ Two stages and a terminal here, against a baseline the compiler
inlines into a single loop: this is the per-stage cost from the section
above, not something particular to `Duplicates`.

`Then` costs nothing per element — it runs once, when the pipeline is
built (`BenchmarkThen_*`).

## Known, priced costs

- **`Zip` and `Equal`** co-iterate two push sequences, which requires
  `iter.Pull` — a runtime coroutine. A zip pipeline measures ~35ns per
  element end to end, which the coroutine dominates. There is no
  cheaper way to do it; the cost is documented rather than hidden.
- **`parallel`** runs callbacks on a pool of goroutines, which is what
  they need and what a hand-written concurrent loop does too. With a
  callback that does nothing, `parallel.Map` over 10k elements takes
  2.6ms — about 0.26µs an element, with no allocation per element —
  against 3.7ms for the hand-written loop (a goroutine per element
  behind a semaphore, results into a preallocated slice) and 0.013ms for
  doing it sequentially. `MapUnordered` and `ForEach` take 3.3–3.7ms. So
  `parallel` costs nothing over writing the concurrency yourself, and
  concurrency costs about a quarter of a microsecond an element over not
  having any: worth it for a callback that waits on I/O, a loss for one
  that does arithmetic. The stages that move the source to its own
  goroutine cost one hand-off an element: about 50ns for `Buffer`, 75ns
  for `Merge` and `ChunkedWithin`. (`parallel/bench_test.go`.)
- **`source.Lines`** yields a line 17% slower than a `bufio.Scanner` loop
  — 3.7ns a line, the cost of handing each one to a consumer through an
  iterator instead of leaving it in the loop. (`source/bench_test.go`.)
- **`Chunked`/`ChunkedBy`/`Windowed`** allocate a fresh slice per emitted
  batch, always. Reusing the buffer would corrupt data for anyone who
  retains a batch; the allocation is the price of that guarantee being
  unconditional.
- **Allocation model**: a k-stage pipeline allocates k closures at
  construction, and streaming operators allocate nothing per element —
  enforced per operator by the conformance suite's allocation checks.

Run everything yourself: `go test -bench . -benchmem ./...`.
