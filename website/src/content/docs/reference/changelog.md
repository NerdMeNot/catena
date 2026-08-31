---
title: "Changelog"
description: "Every released version of catena and what changed in it."
---

All notable changes to catena are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/); versions follow
[semantic versioning](https://semver.org).

## [1.3.0] — 2026-08-31

Documentation and site only — no change to the library's API or behaviour.
The headline is that the docs are now versioned: a reader pinned to an
older release can read that release's documentation instead of the current
one silently replacing it.

### Added

- **Versioned documentation.** The site serves the current release at the
  root and archived releases under their own prefix, with a switcher in
  the header. `v1.1.0` is archived at `/1-1/`, and its pages carry a notice
  saying so. The snapshot is taken from the tag rather than the working
  tree — the versioning plugin archives whatever is currently checked out,
  which would have filed the newer API under the older version's name.
- A 1280×640 social preview card, wired as `og:image` and `twitter:image`
  across every page. Links to the site previously unfurled with no image
  at all. The same file is the repository's GitHub social preview, which
  has no API and is uploaded by hand.

### Changed

- The documentation site is now published from released tags only. Its
  content is derived from the tree it is built from while its version
  label comes from the newest release, so publishing `main` put the two in
  disagreement the moment anything landed unreleased — the site briefly
  documented `BottomN` while announcing itself as v1.1.0, which does not
  have it. A push to `main` now builds the site and runs every check
  against it without deploying. A site-only change can still be published
  from a branch, guarded by a check that refuses unless `docs/operators/`
  matches the latest release tag.
- Canonical URLs, the sitemap and the `og:image` now name
  `catena.nerdmenot.in`. The site answers on both that and its Pages
  subdomain, and naming the subdomain made the branded domain the
  duplicate.
- The version switcher is the site's own component rather than the
  plugin's native `<select>`, whose option list is drawn by the operating
  system and takes none of the site's type or colour. Versions are links,
  so one can be opened in a new tab.
- `API` and `GitHub` in the top navigation open in a new tab, on the
  landing page and in the docs header.
- The README links the documentation site, which it never had.

### Fixed

- The theme toggle lost its selected state on documentation pages. It
  renders twice there — the header and the mobile menu — and both named
  their radios `catena-theme`; radios sharing a name form one group however
  far apart they sit, so the browser kept only the last checked one.
- Switcher links pointed at pages that need not exist in every version.
  The 404 page's switcher offered `/404/` and `/1-1/404/`, neither a
  route. Links now fall back to a version's front page where the same page
  is absent, and `/1-1/` itself is a page rather than a 404.
- `scripts/sync.ts` cleared all of `src/content/docs/` on every run, which
  would have destroyed the archived versions permanently — nothing can
  regenerate a snapshot of a tree that no longer exists. It now clears only
  the directories it generates.

### Removed

- `SPEC.md`. The design document had become a second place where the
  library's contracts were written down, and the two could disagree — as
  they did: its `Try` rule set contradicted the code, its symbol counts had
  drifted, and it specified a conformance harness shape that does not
  exist. Each operator's contract lives in its own doc comment, where godoc
  shows it and the conformance suite enforces it. The document remains in
  the `v1.2.0` tag for anyone who wants the rejected-alternatives history.

## [1.2.0] — 2026-08-31

One new operator and a substantial documentation pass. The pass came out of
an ergonomics and cognitive-load audit whose finding was that the library's
most useful facts were written down where godoc discards them — so nothing
about the API changed, but a good deal more of it is now visible from the
place people read.

### Added

- `BottomN(s, n)` — the no-selector form of `BottomNBy`, completing the
  pair with `TopN`. Without it, wanting the smallest *n* meant either
  `BottomNBy(n, catena.Self[T])` or, more likely, `Sorted(s).Take(n)`,
  which quietly gives up the O(n) memory bound that makes the operator
  worth having. Routes through the same bounded heap, with the same
  stability and tie rules.

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
- The naming law gained the eight suffix patterns it did not cover, its
  known exceptions (`AssociateWith`, `WithIndex`), and the third reason an
  operation lands at package level (receiver shape).

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
  ordering and tie policy, error semantics — specified in each operator's
  doc comment and enforced by the C1–C15 conformance suite, a completeness check that
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

[1.3.0]: https://github.com/NerdMeNot/catena/releases/tag/v1.3.0
[1.2.0]: https://github.com/NerdMeNot/catena/releases/tag/v1.2.0
[1.1.0]: https://github.com/NerdMeNot/catena/releases/tag/v1.1.0
