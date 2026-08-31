---
title: "Changelog"
description: "Every released version of catena and what changed in it."
---

All notable changes to catena are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[semantic versioning](https://semver.org).

## [Unreleased]

Documentation only — no API, behaviour or performance change.

### Fixed

- Four generated `List` mirrors documented themselves as evaluating
  "eagerly" while returning a lazy value: `WithIndex` and `ZipWithNext`
  return a `Seq2`, `MapErr` and `FilterErr` return a `Try`. They now say
  so, and the claim in Concepts that no operator changes evaluation
  strategy names them as the exceptions.
- The eager-operators page claimed `List` carries "the whole `Seq`
  operation set". It carries the whole *method* set: the constraint-bound
  package functions take a `Seq`, so `catena.Sorted(l.AsSeq()).ToList()`
  is the round-trip, and `Concat` needs `l2.AsSeq()`. Both are now
  documented on the `List` type and on the page.
- The package doc pointed at a "constructor table" that did not exist in
  godoc. The normative re-iterability table now lives on the `Seq` type,
  and the nine constructors that said nothing about re-iterability now
  state it — including `Cycle`, which is re-iterable iff its source's
  first pass is.

### Documentation

- `Try`'s five error rules were cited by number eight times in godoc and
  defined nowhere in it: the block was a file-level comment, which godoc
  discards. R1–R5 now live on the `Try` type, along with the rule that
  `Try` is deliberately a small surface — carry errors one stage, then
  commit to a policy and continue on `Seq`. Same fix for `Seq2`'s
  "deliberately absent" list and the `funcs.go` explanation of why
  constraint-bound operations cannot be methods.
- The method-versus-package-function split is now stated in the package
  doc and as a fourth rule in Getting started. It is the first thing that
  bites a newcomer, and the compiler cannot say it: `s.Distinct()`
  produces an error indistinguishable from a typo. The `-By` methods an
  IDE does surface (`SortedBy`, `DistinctBy`, `SumOf`, `MaxBy`, `TopNBy`)
  now name their unsuffixed package-level siblings.
- Added the re-iterability guarantee to Concepts: operators build state
  inside the returned closure, so a chain is re-iterable exactly when its
  root producer is. This turns an open-ended worry into one lookup.
- Added a cross-ecosystem name map (LINQ, Kotlin, Rust) to Getting
  started, an index page for the operator reference, and mutual
  cross-references for the confusable pairs — `MaxBy`/`MaxOf`,
  `Distinct`/`Dedupe`, `Collect`/`ToList`, `Chain`/`Concat`,
  `Once`/`Once1`, `JoinBy`/`Join`.
- The `[S]` marker in the operator catalog read as a warning while
  certifying the opposite; it now says what it means. README notes that
  adopting catena raises a consumer's own `go` directive to 1.27, and its
  two non-compiling snippets are fixed.
- SPEC's naming law gained the eight suffix patterns it did not cover,
  its known exceptions (`AssociateWith`, `WithIndex`), and the third
  reason an operation lands at package level (receiver shape).

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
