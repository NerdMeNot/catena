# Changelog

All notable changes to catena are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[semantic versioning](https://semver.org).

## [1.1.0] — 2026-08-31

The first usable release. `v1.0.0` was published briefly and withdrawn;
it is retracted in `go.mod` and should not be used. Start here.

### The library

- `Seq[T]` — lazy `iter.Seq[T]` with 84 methods (37 chainable), built on Go
  1.27 generic methods; `range` works directly and conversion to and from
  the standard iterator types is free.
- `Try[T]` — fallible sequences where the *consumer* chooses the error
  policy (`Collect` aborts, `CollectAll` gathers, `Ignore` skips), under
  five uniform rules for how errors flow through intermediates.
- `Seq2[K, V]` — the stdlib pair bridge; `List[T]` — the eager mirror,
  generated and conformance-checked to agree with `AsSeq()`.
- The memory-class operators: `FoldBy` (grouped aggregation bounded by
  keys, not elements), `TopNBy`/`BottomNBy` (bounded stable heap),
  `DedupeBy` (O(1) streaming distinct), `ChunkedBy`, `Windowed`, `JoinBy`
  (relational inner join).
- Every operator's contract — argument edges, memory class, drain class,
  ordering and tie policy, error semantics — specified in SPEC.md and
  enforced by the C1–C15 conformance suite, a completeness check that
  fails CI for any unregistered export, property tests, and 100%
  statement coverage of the library.
- An operator reference covering all 182 operators, each with a worked
  example that `go test` runs and whose output it verifies.
- `examples/` — eight runnable programs, all executed in CI.
- Performance measured three ways (hand loop / raw `iter.Seq` closures /
  catena) with the standing rule that catena meets the raw mechanism
  wherever the two compile on equal terms — it wins on multi-stage
  pipelines and terminals, and trails by 4–13% on single-stage shapes,
  where the baseline inlines into its consumer and a library call cannot.
  Operators compose by direct source invocation rather than
  `range`, which makes a 4-stage pipeline 25% faster than the same
  pipeline hand-built from raw closures.

[1.1.0]: https://github.com/NerdMeNot/catena/releases/tag/v1.1.0
