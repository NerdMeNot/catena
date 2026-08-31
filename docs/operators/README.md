# Operator reference

Every operator, with its signature, its memory and termination
behaviour, and a worked example that `go test` runs and verifies.
For the one-line-per-operator index with costs at a glance, see
[the catalog](../04-operators.md).

1. [Filtering](01-filtering.md) — Keeping some elements and dropping others — by predicate, by key, or by what has already been seen
2. [Creating a sequence](02-creating.md) — Where a pipeline starts
3. [Slicing](03-slicing.md) — Taking a window of the sequence by position or by a predicate
4. [Transforming](04-transforming.md) — Changing what the elements are
5. [Combining](05-combining.md) — Putting two or more sequences together, or pairing a sequence with itself
6. [Ordering](06-ordering.md) — Sorting and reversing
7. [Batching](07-batching.md) — Grouping consecutive elements into slices — for bulk writes, paged requests, or moving windows
8. [Consuming](08-consuming.md) — The operators that make a pipeline run
9. [Searching and testing](09-searching.md) — Asking a question of a sequence rather than transforming it
10. [Folding and grouping](10-folding.md) — Reducing a sequence to one value, or to one value per key
11. [Aggregating](11-aggregating.md) — Extremes, totals and joins
12. [Errors](12-errors.md) — Try is a sequence whose elements each either succeeded or carry an error, and its whole design is that the pipeline does not decide what a failure means — the consumer does, by choosing a terminal
13. [Pairs](13-pairs.md) — Seq2 is the standard library's pairing type with methods — what maps.All and slices.All speak, so every boundary with the standard library is a free conversion
14. [Eager](14-eager.md) — List is a []T carrying the whole Seq *method* set, evaluated at once with exact preallocation
