# Examples

Each directory is a runnable program: `go run ./examples/<name>`. They are
ordered — the early ones introduce the mechanics, the later ones show the
patterns the library exists for.

| Example | What it shows |
|---|---|
| [basics](01-basics/main.go) | building pipelines, laziness, early termination, `range` interop |
| [grouping](02-grouping/main.go) | `FoldBy` with struct accumulators, composite keys, `TallyBy`, `IndexBy` |
| [selection](03-selection/main.go) | `TopNBy`, `SortedBy`, ties and stability, `MinMaxOf` |
| [errors](04-errors/main.go) | `Try`: one parse pipeline, three error policies, `WrapErr`, `Recover` |
| [resources](05-resources/main.go) | producers that own a resource: lazy acquisition, cleanup on early exit, `UntilDone` |
| [streaming](06-streaming/main.go) | infinite sequences, `Scan` running state, `Windowed` moving averages, `Chunked` batching |
| [join](07-join/main.go) | `JoinBy`: relational joins between struct tables, then rollups on the joined stream |
| [list](08-list/main.go) | eager `List`, crossing between eager and lazy, the `Seq2` bridge |

None of them import anything beyond the standard library and catena, and
every one prints what it computes — reading the output next to the code is
the point.
