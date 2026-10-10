---
title: "Linting"
description: "Most of what goes wrong in a pipeline is not a bug. The code is correct; it just sorts a million elements to keep ten, or builds a pipeline and never runs it. catenalint finds those."
sidebar:
  order: 7
---

Most of what goes wrong in a pipeline is not a bug. The code is correct;
it just sorts a million elements to keep ten, or builds a pipeline and
never runs it. `catenalint` finds those.

```sh
go install github.com/NerdMeNot/catena/lint/cmd/catenalint@latest
go vet -vettool=$(which catenalint) ./...
```

It is a set of ordinary `go/analysis` analyzers, so it also runs
standalone (`catenalint ./...`, with `-fix` to apply the fixes) and can be
loaded by anything that hosts analyzers.

The linter is a separate module, `github.com/NerdMeNot/catena/lint`. It
needs `golang.org/x/tools`, and the library itself depends on nothing:
keeping them apart means adding catena to a project adds exactly one
module.

## The checks

| Check | Reports | Suggests |
|---|---|---|
| `catenatopn` | `Sorted(s).Take(n)`, `s.SortedBy(sel).Take(n)` and their `Desc` forms | `BottomN`, `BottomNBy`, `TopN`, `TopNBy` |
| `catenaminmax` | `Sorted(s).First()`, `.Last()`, `s.SortedBy(sel).First()`, `s.SortedWith(cmp).First()` | `Min`, `Max`, `MinBy`, `MaxBy`, `MinWith` |
| `catenadedupe` | `Distinct(Sorted(s))` | `Dedupe` |
| `catenatally` | `for _, bucket := range s.GroupBy(sel)` where the loop only uses `len(bucket)` | `TallyBy` |
| `catenaunusedseq` | a `Seq`, `Seq2` or `Try` built as a statement and discarded | — |

The first four are about memory class. Sorting buffers the whole input;
`TopN` holds `n` elements, `Min` holds one. `Distinct` remembers every
value it has seen; on sorted input `Dedupe` remembers the last. `GroupBy`
keeps every element; `TallyBy` keeps a count per key.

The last is about laziness. A pipeline is a description of work, and
nothing in it runs until a terminal consumes it:

```go
orders.OnEach(audit)         // reported: nothing ever calls audit
orders.OnEach(audit).Drain() // runs
```

Both spellings of a package function are recognised — `catena.Sorted(s)`
and `s.Then(catena.Sorted)`.

## What a fix promises

A check offers a fix only where the replacement returns **the same
values of the same type**, including which of several equal elements you
get. Where that does not hold it reports without fixing, or stays silent:

- `Sorted(s).Take(n)` is a sequence and `TopN(s, n)` is a slice. The fix
  is offered when the sequence is collected on the spot —
  `Sorted(s).Take(n).Collect()` — and only the report otherwise.
- `s.SortedBy(sel).First()` is fixed to `s.MinBy(sel)`: a stable sort
  puts the earliest of several equal keys first, and that is the one
  `MinBy` returns. `s.SortedBy(sel).Last()` is not reported at all — it
  is the *latest* of the equal keys, and `MaxBy` returns the earliest.
- `catenatally` recognises one shape and no others. A `GroupBy` stored in
  a variable, passed along, or whose buckets are read is left alone.

So a clean run is not a proof that nothing is wasteful. It means none of
the shapes the linter can be sure about are present.

## Silencing a report

Assign the result: `_ = s.Map(f)` is not reported by `catenaunusedseq`.
For the others, a `//nolint`-style directive is whatever your runner
supports; `go vet` has none, and each check can be switched off by name —
`-catenatopn=false`.

## Versions

The linter is tagged on its own line, `lint/v0.x.y`, and is still at v0:
checks may be added, and a report may gain or lose a fix, in a minor
version. It matches catena by package path rather than by importing it,
so one linter version works across catena releases; a check for an
operator your catena version does not have simply never fires.

The library's own examples are linted in CI and must come back clean.
