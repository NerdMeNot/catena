# Coming from lo, Kotlin or LINQ

You already know what you want; you need what catena calls it. Each table
lists the name you know and the catena operator that does the same job,
with a note where the two differ in a way that matters.

Two things are different throughout, so the tables do not repeat them:

- **Constraint-bound operators are package functions.** Anything that
  requires something of the element type — `Distinct`, `Sorted`, `Sum`,
  `Max` — is `catena.Sorted(s)`, not `s.Sorted()`. `Then` keeps the chain
  unbroken: `s.Then(catena.Sorted)`.
- **"Not found" is a second result, not a zero value or an exception.**
  `First` returns `(T, bool)`; `FirstOr` takes the fallback instead.

Every catena name on this page is checked against the library by a test,
so the page cannot drift from the API.

## From lo

`lo` works on slices and is eager; catena is lazy. `catena.FromSlice(xs)`
enters, `Collect()` leaves, and `List` is the eager form with the same
methods.

| lo | catena | Note |
|---|---|---|
| `Filter`, `Reject` | `Filter`, `FilterNot` | |
| `Map`, `FilterMap`, `FlatMap` | `Map`, `FilterMap`, `FlatMapSlice` | `FlatMap` takes a function returning a sequence |
| `Reduce` | `Fold` | `Reduce` in catena takes no initial value |
| `ForEach`, `ForEachWhile` | `ForEach`, `All` | |
| `Uniq`, `UniqBy` | `Distinct`, `DistinctBy` | |
| `FindDuplicates`, `FindDuplicatesBy` | `Duplicates`, `DuplicatesBy` | yields each at its second occurrence |
| `GroupBy` | `GroupBy` | a map; `GroupedBy` keeps first-seen order |
| `PartitionBy` | `GroupedBy` | |
| `Chunk`, `Sliding` | `Chunked`, `Windowed` | |
| `Flatten` | `FlattenSlices` | `Flatten` is for a sequence of sequences |
| `Reverse` | `Reversed` | |
| `KeyBy`, `Associate` | `IndexBy`, `Associate` | |
| `CountBy`, `CountValues`, `CountValuesBy` | `CountWhere`, `Tally`, `TallyBy` | |
| `Compact` | `NonZero` | |
| `Drop`, `DropRight`, `DropWhile` | `Drop`, `DropLast`, `DropWhile` | |
| `Find`, `IndexOf`, `Nth` | `Find`, `IndexOf`, `ElementAt` | |
| `First`, `Last`, `FirstOr`, `LastOr` | `First`, `Last`, `FirstOr`, `LastOr` | |
| `Min`, `Max` | `Min`, `Max` | |
| `MinBy`, `MaxBy` | `MinWith`, `MaxWith` | lo's take a comparison; catena's `MinBy` takes a key |
| `Sum`, `SumBy`, `Mean` | `Sum`, `SumOf`, `Average` | |
| `Contains`, `ContainsBy` | `Contains`, `Any` | |
| `EveryBy`, `SomeBy`, `NoneBy` | `All`, `Any`, `None` | |
| `Union`, `Intersect`, `Without` | `Union`, `Intersect`, `Except` | |
| `IsSorted`, `IsSortedByKey` | `IsSorted`, `IsSortedBy` | |
| `Sample`, `Samples` | `Sample` | reservoir sampling: O(n) memory |
| `Zip2`, `Unzip2` | `Zip`, `Unzip` | pairs are a `Seq2`, not a tuple |
| `Range`, `Repeat` | `Range`, `RepeatN` | |
| `Keys`, `Values` | `Keys`, `Values` | on `catena.FromMap(m)` |
| `Must` | `Must` | on a `Try` |
| `lop.Map`, `lop.ForEach` | `parallel.Map`, `parallel.ForEach` | bounded, ordered, and joined before returning |
| `FanIn`, `BufferWithTimeout` | `parallel.Merge`, `parallel.ChunkedWithin` | sequences rather than channels |

Not here, on purpose: `Ternary`, `ToPtr`, `Coalesce` and the string
helpers (catena is only about sequences), tuples beyond pairs, and the
map-rewriting helpers such as `PickBy` and `Invert`.

## From Kotlin

`Seq` is `Sequence`: lazy, with the same split between intermediate and
terminal operations. `List` is `List`.

| Kotlin | catena | Note |
|---|---|---|
| `filter`, `filterNot`, `filterIndexed` | `Filter`, `FilterNot`, `FilterIndexed` | |
| `map`, `mapIndexed`, `mapNotNull` | `Map`, `MapIndexed`, `FilterMap` | |
| `flatMap`, `flatten` | `FlatMap`, `Flatten` | |
| `fold`, `reduce`, `runningFold` | `Fold`, `Reduce`, `Scan` | |
| `take`, `takeWhile`, `takeLast` | `Take`, `TakeWhile`, `TakeLast` | |
| `drop`, `dropWhile`, `dropLast` | `Drop`, `DropWhile`, `DropLast` | |
| `distinct`, `distinctBy` | `Distinct`, `DistinctBy` | |
| `sorted`, `sortedDescending`, `sortedBy`, `sortedByDescending`, `sortedWith` | `Sorted`, `SortedDesc`, `SortedBy`, `SortedByDesc`, `SortedWith` | |
| `groupBy` | `GroupBy` | a map; `GroupedBy` keeps first-seen order |
| `groupingBy { }.fold`, `eachCount` | `FoldBy`, `TallyBy` | `FoldedBy` stays in the chain |
| `associate`, `associateBy`, `associateWith` | `Associate`, `IndexBy`, `AssociateWith` | |
| `partition` | `Partition` | |
| `chunked`, `windowed`, `zipWithNext` | `Chunked`, `Windowed`, `ZipWithNext` | |
| `zip`, `unzip`, `withIndex` | `Zip`, `Unzip`, `WithIndex` | |
| `first`, `firstOrNull`, `last`, `single` | `First`, `FirstOr`, `Last`, `Single` | `First` returns `(T, bool)` |
| `find`, `findLast`, `indexOfFirst`, `elementAt` | `Find`, `FindLast`, `FindIndex`, `ElementAt` | |
| `any`, `all`, `none`, `count` | `Any`, `All`, `None`, `CountWhere` | `Count` takes no predicate |
| `sumOf`, `maxOf`, `minOf` | `SumOf`, `MaxOf`, `MinOf` | |
| `maxBy`, `minBy`, `maxWith`, `minWith` | `MaxBy`, `MinBy`, `MaxWith`, `MinWith` | |
| `joinToString` | `JoinToString` | |
| `onEach`, `forEach`, `forEachIndexed` | `OnEach`, `ForEach`, `ForEachIndexed` | |
| `generateSequence` | `Generate`, `GenerateWhile` | |
| `plus` | `Concat`, `Append` | |
| `ifEmpty` | `IfEmpty` | |
| `toList` | `Collect`, `ToList` | `Collect` gives a slice |
| `let` | `Then` | |
| `runCatching`, `Result` | `MapErr`, `UntilErr` | errors are per element, on a `Try` |
| Flow's `buffer`, `merge` | `parallel.Buffer`, `parallel.Merge` | |
| Flow's `flatMapMerge` over a suspending call | `parallel.Map`, `parallel.MapUnordered` | |

## From LINQ

`Seq` is `IEnumerable<T>` with deferred execution. The query operators
that take a key selector end in `By` here.

| LINQ | catena | Note |
|---|---|---|
| `Where`, `Select`, `SelectMany` | `Filter`, `Map`, `FlatMap` | |
| `Aggregate` | `Fold`, `Reduce` | with and without a seed |
| `Take`, `TakeWhile`, `TakeLast` | `Take`, `TakeWhile`, `TakeLast` | |
| `Skip`, `SkipWhile`, `SkipLast` | `Drop`, `DropWhile`, `DropLast` | |
| `Distinct`, `DistinctBy` | `Distinct`, `DistinctBy` | |
| `OrderBy`, `OrderByDescending` | `SortedBy`, `SortedByDesc` | |
| `ThenBy` | `SortedBy` | sorts are stable: sort by the minor key first, then the major |
| `Reverse` | `Reversed` | |
| `GroupBy` | `GroupedBy` | ordered and chainable, as LINQ's is |
| `ToLookup`, `ToDictionary` | `GroupBy`, `IndexBy` | |
| `CountBy`, `AggregateBy` | `TallyBy`, `FoldBy` | |
| `Join`, `GroupJoin` | `JoinBy`, `GroupJoinBy` | |
| `GroupJoin` + `DefaultIfEmpty` | `LeftJoinBy` | the left outer join, in one operator |
| `Chunk`, `Zip` | `Chunked`, `Zip` | |
| `First`, `FirstOrDefault`, `Last`, `LastOrDefault` | `First`, `FirstOr`, `Last`, `LastOr` | |
| `Single`, `ElementAt` | `Single`, `ElementAt` | |
| `Any`, `All`, `Count`, `Contains` | `Any`, `All`, `CountWhere`, `Contains` | |
| `Sum`, `Average`, `Min`, `Max`, `MinBy`, `MaxBy` | `SumOf`, `AverageOf`, `MinOf`, `MaxOf`, `MinBy`, `MaxBy` | the selector-free forms are package functions |
| `Concat`, `Append`, `Prepend` | `Concat`, `Append`, `Prepend` | |
| `Union`, `Intersect`, `Except` | `Union`, `Intersect`, `Except` | |
| `DefaultIfEmpty` | `IfEmpty` | |
| `SequenceEqual` | `Equal` | |
| `Range`, `Repeat`, `Empty` | `Range`, `RepeatN`, `Empty` | |
| `ToList`, `ToArray` | `ToList`, `Collect` | |
| `AsParallel().Select` | `parallel.Map` | order is kept; `parallel.MapUnordered` is the `AsUnordered` form |
