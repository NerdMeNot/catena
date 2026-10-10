---
title: "Operator catalog"
description: "Every operator, grouped the way the source is. Full signatures are on pkg.go.dev; this page is for finding the right operator and knowing its costs at a glance."
sidebar:
  order: 4
---

Every operator, grouped the way the source is. Full signatures are on
[pkg.go.dev](https://pkg.go.dev/github.com/NerdMeNot/catena); this page is
for finding the right operator and knowing its costs at a glance.

Markers: **⚠ mem** buffers beyond O(1), with its bound · **⚠∞** fully
drains — hangs on infinite input · **◐** conditional drain — terminates only
when the answer arrives · **[S]** builds per-iteration state, created fresh
on each pass and verified re-iterable by the conformance suite — a note, not
a warning.

## Constructors

| Operator | Notes |
|---|---|
| `Of`, `FromSlice` | re-iterable; the slice is not copied |
| `From`, `From2`, `FromErrs` | adapt any push-function iterator, free |
| `FromMap` | re-iterable; map order |
| `FromChan(ctx, ch)` | single-use; no goroutine; stops on close or cancel |
| `Empty`, `Empty2`, `EmptyTry`, `Once1` | the trivial sequences |
| `Repeat`, `RepeatN` | infinite / n copies |
| `Generate`, `GenerateWhile` | seed first, always; infinite / until false |
| `Range(start, stop, step)` | overflow-guarded; sign mismatch → empty; step 0 panics |
| `Cycle` | infinite; ⚠ mem all (replay buffer); empty input terminates |

## Type-preserving intermediates (`seq.go`)

| Operator | Notes |
|---|---|
| `Filter`, `FilterNot`, `FilterIndexed` | |
| `Take`, `Drop` | [S] consume exactly what they emit / skip |
| `TakeWhile`, `DropWhile` | [S] |
| `TakeLast(n)` | [S] ⚠ mem n, ⚠∞ drains before emitting |
| `DropLast(n)` | [S] ⚠ mem n; streams with an n-element lag |
| `Step(n)` | [S] first element, then every nth |
| `OnEach` | side effects, passthrough |
| `Concat`, `Append`, `Prepend` | |
| `Intersperse(sep)` | [S] |
| `IfEmpty(defaults...)` | [S] defaults only if the source yields nothing |
| `SortedWith` | ⚠ mem all, ⚠∞; stable |
| `DistinctWith` | [S] ⚠ mem distinct; O(n²) compares — small inputs |
| `Reversed` | ⚠ mem all, ⚠∞ |
| `Once` | panics on second consumption — a development guard |
| `IsSortedWith(cmp)` | ◐ stops at the first pair out of order |
| `Then(f)` | returns `f(s)` — chains package functions and `parallel` stages |
| `UntilDone(ctx)` | → `Try`; yields ctx.Err() and stops on cancellation |

## Type-changing intermediates (`seq_map.go`)

| Operator | Notes |
|---|---|
| `Map`, `MapIndexed` | |
| `FilterMap` | fused filter+map, comma-ok |
| `FlatMap`, `FlatMapSlice` | |
| `Scan(init, f)` | [S] running fold; init itself is not yielded |
| `WithIndex` | → `Seq2[int, T]`, from 0 |
| `ZipWithNext` | → `Seq2[T, T]`; empty/single input → empty |
| `Zip(other)` | → `Seq2`; stops at the shorter side; other via `iter.Pull` |
| `MapErr`, `FilterErr` | → `Try`; a failed callback yields (zero, err) |
| `DistinctBy(sel)` | [S] ⚠ mem distinct keys; first wins |
| `DuplicatesBy(sel)` | [S] ⚠ mem distinct keys; each repeated key once, at its second element |
| `IsSortedBy(sel)` | ◐ stops at the first pair out of order; sel once per element |
| `DedupeBy(sel)` | [S] O(1); consecutive duplicates only |
| `SortedBy`, `SortedByDesc` | ⚠ mem all, ⚠∞; stable; sel once per element |
| `JoinBy` | inner hash join; ⚠ mem right side, buffered before first emission |
| `LeftJoinBy` | left outer join: an unmatched left element is kept, and `combine` is told; ⚠ mem right side |
| `GroupJoinBy` | one output per left element, with all its matches together (shared elements — do not assign to them); ⚠ mem right side |

## Consumption and search terminals (`seq_terminal.go`)

| Operator | Notes |
|---|---|
| `Collect`, `ToList` | ⚠∞; nil for empty |
| `Seq()` | free conversion to `iter.Seq` |
| `Pull()` | caller MUST call stop, or the producer's cleanup never runs |
| `ToChan(ctx)` | unbuffered; goroutine ends on drain or cancel |
| `ForEach`, `ForEachIndexed`, `Drain` | ⚠∞ |
| `ForEachErr` | ◐ stops at the first error f returns |
| `First` | |
| `FirstOr(fallback)`, `LastOr(fallback)` | the element, or the fallback when empty; `LastOr` is ⚠∞ |
| `Last`, `FindLast` | ⚠∞ |
| `Single` | ◐ true iff exactly one; stops at the second element |
| `ElementAt(i)` | negative i → (zero, false), consumes nothing |
| `Find`, `FindIndex`, `FindMap` | ◐ |
| `Any`, `All`, `None` | ◐; `All` vacuously true on empty |
| `Count`, `CountWhere` | ⚠∞ |
| `IsEmpty` | consumes ONE element — lost on a single-pass source |

## Folds and grouping (`seq_fold.go`)

All ⚠∞; map results iterate in undefined order.

| Operator | Notes |
|---|---|
| `Fold`, `FoldIndexed` | |
| `FoldWhile` | ◐ the stopping call's accumulator is included |
| `FoldErr` | ◐ accumulator-so-far + first error |
| `FoldBy(key, init, f)` | ⚠ mem distinct keys — the streaming GroupBy |
| `FoldedBy(key, init, f)` | → `Seq2`, keys in first-seen order — FoldBy that stays in the chain; ⚠ mem distinct keys, ⚠∞ |
| `Reduce` | first element as initial accumulator; (zero, false) on empty |
| `GroupBy` | ⚠ mem all; buckets in encounter order |
| `IndexBy`, `Associate` | last wins on key collisions |
| `TallyBy` | counts per key |
| `Partition` | both sides in encounter order |

## Aggregation (`seq_aggregate.go`)

All ⚠∞; `cmp.Compare` ordering (NaN below everything); ties first-wins;
(zero, false) on empty.

| Operator | Notes |
|---|---|
| `MaxBy`, `MinBy` | the *element* with the extreme key |
| `MaxOf`, `MinOf`, `MinMaxOf` | the extreme *key*; MinMaxOf in one pass |
| `MaxWith`, `MinWith` | comparator forms |
| `TopNBy`, `BottomNBy` | ⚠ mem n; sorted output; stable at the cut |
| `Sample(n, rng)` | ⚠∞ ⚠ mem n; n elements uniformly at random, in encounter order; nil rng is the shared generator |
| `SumOf`, `ProductOf` | wraps like +; empty product is 1 |
| `AverageOf` | float64 accumulation; (0, false) on empty |
| `JoinToString` | |

## Package functions (`funcs.go`)

Constraint-bound — these cannot be methods because they require something
of `T`.

| Operator | Notes |
|---|---|
| `Distinct`, `Dedupe` | [S]; first wins / consecutive only |
| `Duplicates` | [S] ⚠ mem distinct; each repeated value once, at its second occurrence |
| `IsSorted` | ◐ stops at the first pair out of order |
| `Sorted`, `SortedDesc` | ⚠ mem all, ⚠∞; stable |
| `Sum`, `Product`, `Average`, `Max`, `Min`, `MinMax`, `TopN`, `BottomN` | ⚠∞ |
| `Contains`, `IndexOf` | ◐ |
| `NonZero` | drops zero values |
| `ToKeySet`, `Tally`, `AssociateWith` | ⚠∞ |
| `Equal` | consumes both to the first difference; b via `iter.Pull` |
| `Union`, `Intersect`, `Except` | set semantics, encounter order; ⚠ mem |
| `Flatten`, `FlattenSlices`, `Chain` | |
| `Unzip`, `CollectMap`, `Join` | ⚠∞ |

## Batch operators (`batch.go`)

Package functions of necessity — a `Seq[T]` method returning `Seq[[]T]`,
or anything that leads back to it, is an instantiation cycle. Every
emitted slice is fresh; retaining one is always safe.

| Operator | Notes |
|---|---|
| `Chunked(s, n)` | [S]; last chunk may be partial |
| `ChunkedBy(s, sel)` | [S] ⚠ mem longest run; a chunk per run of equal keys |
| `Windowed(s, size, step)` | [S] ⚠ mem size; full windows only; step > size samples |
| `GroupedBy(s, sel)` | → `Seq2[K, []T]`, keys in first-seen order — GroupBy that stays in the chain; ⚠ mem all, ⚠∞ |

## Concurrency (`catena/parallel`)

The operators that start goroutines, as stages for `Seq.Then`. Every one
joins its goroutines before its iteration returns. See
[06-concurrency.md](/guides/concurrency/).

| Operator | Notes |
|---|---|
| `parallel.Map(ctx, n, f)` | → `Try`; `n` calls at once, results in source order; ⚠ mem `n` results; source stays on the consumer's goroutine |
| `parallel.MapUnordered(ctx, n, f)` | → `Try`; results as calls finish; ⚠ mem `n` |
| `parallel.ForEach(ctx, n, f)` | terminal; first error, the rest cancelled and joined; ⚠∞ on success |
| `parallel.Buffer[T](n)` | read-ahead on a goroutine of its own; ⚠ mem `n`; upstream runs concurrently with the consumer |
| `parallel.ChunkedWithin[T](n, d)` | → `Seq[[]T]`; a chunk at `n` elements or `d` after its first; never empty; ⚠ mem `n` |
| `parallel.Merge(seqs...)` | interleaves by arrival, one goroutine per input; order within an input is kept |

## Sources (`catena/source`, `catena/source/sqlsource`)

Producers that open their resource inside the iteration and release it on
every exit. All return a `Try`.

| Operator | Notes |
|---|---|
| `source.Lines(open)` | lines of a reader, any length, endings stripped; ⚠ mem one line |
| `source.Paginate(first, fetch)` | a page is fetched only once the one before it is yielded in full; ⚠ mem one page |
| `sqlsource.Rows(open, scan)` | rows of a query; a failed scan is an element and iteration continues |

## `Seq2`, `Try`, `List`

`Seq2` carries the bridge set: `Filter`, `Map`, `MapValues`, `MapTo`,
`Take`, `Drop`, `Keys`, `Values`, `Swap`, `Fold`, `ForEach`, `Any`, `All`,
`Count`, `First`, `Seq2`, `Pull`. To a map: `catena.CollectMap`. To two
slices: `catena.Unzip`.

`Try` mirrors the intermediates under the five error rules
([03-error-handling.md](/guides/error-handling/)) and adds the policy terminals:
`Collect`, `CollectAll`, `Ignore`, `Errs`, `Must`, `Err`, plus `Recover`,
`WrapErr`, `OnError`, `UntilDone`.

`List` mirrors the whole `Seq` operation set eagerly (generated, and
conformance-checked to agree with `AsSeq()`), plus `Len`, `At`, `Get`,
`Slice`, `Clone`, `AsSeq`, `FoldRight`, and a never-aliasing `Append`.
