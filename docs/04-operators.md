# Operator catalog

Every operator, grouped the way the source is. Full signatures are on
[pkg.go.dev](https://pkg.go.dev/github.com/NerdMeNot/catena); this page is
for finding the right operator and knowing its costs at a glance.

## I want to…

The operators people reach for most, by the job rather than the name.

| I want to | Use |
|---|---|
| Keep some elements, change each one | [`Filter`](operators/01-filtering.md#filter), [`Map`](operators/04-transforming.md#map) |
| Drop duplicates | [`Distinct`](operators/01-filtering.md#distinct), [`DistinctBy`](operators/01-filtering.md#distinctby) |
| Get the top or bottom N without sorting everything | [`TopN`](operators/11-aggregating.md#topn), [`TopNBy`](operators/11-aggregating.md#topnby) |
| Count, sum or group by a key | [`TallyBy`](operators/10-folding.md#tallyby), [`GroupBy`](operators/10-folding.md#groupby), [`FoldedBy`](operators/10-folding.md#foldedby) |
| Split into two by a condition | [`Partition`](operators/10-folding.md#partition) |
| Process in batches of N | [`Chunked`](operators/07-batching.md#chunked) |
| Batch a live stream by size or time | [`parallel.ChunkedWithin`](operators/15-concurrency.md#chunkedwithin) |
| Match two lists by a key | [`JoinBy`](operators/05-combining.md#joinby), [`LeftJoinBy`](operators/05-combining.md#leftjoinby) |
| Read a file line by line, a CSV, or JSON Lines | [`source.Lines`](operators/16-sources.md#lines), [`source.CSVHeader`](operators/16-sources.md#csvheader), [`source.JSONLines`](operators/16-sources.md#jsonlines) |
| Read the rows of a query, the pages of an API | [`sqlsource.Rows`](operators/16-sources.md#rows), [`source.Paginate`](operators/16-sources.md#paginate) |
| Keep going past bad records, or stop at the first | [`CollectAll`](operators/12-errors.md#collectall), [`Collect`](operators/12-errors.md#collect) |
| Call a function for many items, several at once | [`parallel.Map`](operators/15-concurrency.md#map), [`parallel.MapTry`](operators/15-concurrency.md#maptry) |
| Retry a flaky call, or keep to a rate limit | [`parallel.Retry`](operators/15-concurrency.md#retry) with [`parallel.Backoff`](operators/15-concurrency.md#backoff), [`parallel.Limit`](operators/15-concurrency.md#limit) |
| Run a few different calls at once | [`parallel.Do`](operators/15-concurrency.md#do) |

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

The operators that start goroutines, as stages for `Seq.Then` and
`Try.Then`, and `Do`; and the callback wrappers `Retry` and `Limit`. Every one
joins its goroutines before its iteration returns. See
[06-concurrency.md](06-concurrency.md).

| Operator | Notes |
|---|---|
| `parallel.Map(ctx, n, f)` | → `Try`; `n` calls at once, results in source order; ⚠ mem `n` results; source stays on the consumer's goroutine |
| `parallel.MapUnordered(ctx, n, f)` | → `Try`; results as calls finish; ⚠ mem `n` |
| `parallel.ForEach(ctx, n, f)` | terminal; first error, the rest cancelled and joined; ⚠∞ on success |
| `parallel.MapTry`, `MapUnorderedTry`, `ForEachTry` | the same, from a `Try` via `Try.Then`: an errored element makes no call — `MapTry` keeps it in place, `MapUnorderedTry` yields it at once, `ForEachTry` treats it as a failed call |
| `parallel.Do(ctx, fns...)` | runs different functions at once; first error, the rest cancelled and joined |
| `parallel.Retry(attempts, backoff, retryable, f)` | wraps a callback: tries again, waiting `backoff(k, err)`, for the errors you say; no goroutines |
| `parallel.Backoff(base, ceiling)` | the usual backoff for `Retry`: exponential, capped, with jitter |
| `parallel.Limit(n, per, f)` | wraps a callback: at most `n` calls let through per `per`, shared by every use; no goroutines |
| `parallel.Buffer[T](n)` | read-ahead on a goroutine of its own; ⚠ mem `n`; upstream runs concurrently with the consumer |
| `parallel.ChunkedWithin[T](n, d)` | → `Seq[[]T]`; a chunk at `n` elements or `d` after its first; never empty; ⚠ mem `n` |
| `parallel.Merge(seqs...)` | interleaves by arrival, one goroutine per input; order within an input is kept |

## Sources (`catena/source`, `catena/source/sqlsource`)

Producers that open their resource inside the iteration and release it on
every exit. All return a `Try`.

| Operator | Notes |
|---|---|
| `source.Lines(open)` | lines of a reader, any length, endings stripped; ⚠ mem one line |
| `source.CSV(open, setup)` | records of CSV data with no header; a malformed record is an element and reading continues; ⚠ mem one record |
| `source.CSVHeader(open, setup)` | CSV with a header row: each record a `Record`, its fields by column name; a header that cannot be read ends it |
| `source.JSONLines[T](open, setup)` | one JSON value per line; a line that does not decode is an element and reading continues; ⚠ mem one line |
| `source.Paginate(first, fetch)` | a page is fetched only once the one before it is yielded in full; ⚠ mem one page |
| `sqlsource.Rows(open, scan)` | rows of a query; a failed scan is an element and iteration continues |

## `Seq2`, `Try`, `List`

`Seq2` carries the bridge set: `Filter`, `Map`, `MapValues`, `MapTo`,
`Take`, `Drop`, `Keys`, `Values`, `Swap`, `Fold`, `ForEach`, `Any`, `All`,
`Count`, `First`, `Seq2`, `Pull`. To a map: `catena.CollectMap`. To two
slices: `catena.Unzip`.

`Try` mirrors the intermediates under the five error rules
([03-error-handling.md](03-error-handling.md)) and adds the policy terminals:
`Collect`, `CollectAll`, `Ignore`, `Errs`, `UntilErr`, `Must`, `Err`, plus `Recover`,
`WrapErr`, `OnError`, `UntilDone`, and `Then` for functions of a whole
`Try`, such as the `parallel` Try stages.

`List` mirrors the whole `Seq` operation set eagerly (generated, and
conformance-checked to agree with `AsSeq()`), plus `Len`, `At`, `Get`,
`Slice`, `Clone`, `AsSeq`, `FoldRight`, and a never-aliasing `Append`.
