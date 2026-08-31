<div align="center">

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="Assets/icon-dark.svg">
  <img alt="" src="Assets/icon.svg" width="96" height="96">
</picture>

# catena

**Lazy, fully typed sequence pipelines for Go.**
Kotlin-stdlib and LINQ ergonomics on top of `iter.Seq`, built on Go 1.27 generic methods.

[![CI](https://github.com/NerdMeNot/catena/actions/workflows/ci.yml/badge.svg)](https://github.com/NerdMeNot/catena/actions/workflows/ci.yml)
[![Coverage 100%](https://img.shields.io/badge/coverage-100%25-brightgreen)](.github/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/NerdMeNot/catena?label=release&sort=semver&color=00ADD8)](https://github.com/NerdMeNot/catena/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/NerdMeNot/catena.svg)](https://pkg.go.dev/github.com/NerdMeNot/catena)
[![Go 1.27+](https://img.shields.io/badge/go-1.27%2B-00ADD8)](go.mod)
[![License: Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue)](LICENSE)

</div>

```go
// Order and UserID are your own types; orders is a []Order.
top := catena.FromSlice(orders).
    Filter(func(o Order) bool { return o.Paid }).
    TopNBy(10, func(o Order) int { return o.Amount })

byUser := catena.FromSlice(orders).FoldBy(
    func(o Order) UserID { return o.User },
    func(UserID) int { return 0 },
    func(sum int, o Order) int { return sum + o.Amount },
)
```

```sh
go get github.com/NerdMeNot/catena
```

Requires **Go 1.27+** for generic methods. Adding catena therefore raises
your module's own `go` directive to 1.27, and everyone building your code
— teammates and CI included — needs a 1.27 toolchain. No runtime
dependencies: the one `require` line is a test-only property-testing
library, and `go list -deps` on the package resolves to the standard
library alone.

*Catena* is Latin for "chain".

---

## Why

Go finally has the two pieces this needed — `iter.Seq` as a common
iterator currency, and generic methods so a chain can change element type
mid-flight. What was missing was a library willing to be precise about the
things that make or break iterator code in practice.

**Nothing is hidden.** No `reflect`, no `any` in value position, no hidden
buffering, and exactly one goroutine in the library — `ToChan`'s, declared
in the first line of its doc comment. An operator that buffers says so
too, with its bound; a terminal that would hang on an infinite sequence is
marked; an invalid argument panics at construction, naming the operator
you actually called. The only panics during iteration are `Once`'s and
`Must`'s, which is what those two are for.

**The contracts are tested, not promised.** Every exported operator is
registered in a conformance harness, compared against a hand-written loop,
and put through the invariants its class allows: re-iteration, early
termination over an infinite source, the producer's cleanup running on an
early break, and a bound on how much of the source it may consume to
produce k elements. A completeness check parses the package and fails CI
for any operator nothing registers. Library statement coverage is 100%,
and the badge is not a snapshot: CI recomputes it on every push and fails
the build below 100%, so a regression turns the build red rather than the
badge stale.

**Slow only where Go is slow.** Measured against the same pipelines
hand-built from raw `iter.Seq` closures, catena beats the raw mechanism on
multi-stage pipelines and on terminals, and costs 4–13% more on
single-stage shapes — where the baseline is inlined into its consumer and
a call across a package boundary cannot be. What remains against a plain
loop is the iterator protocol itself, and every number is published rather
than waved at.

## In practice

Pipelines stay lazy until a terminal consumes them, and early termination
propagates through every stage — `Take(2)` here computes exactly two
squares, out of an infinite sequence:

```go
catena.Generate(1, func(n int) int { return n * 2 }). // 1, 2, 4, 8, ...
    Map(func(n int) int { return n * n }).
    Take(2).
    Collect() // [1 4]
```

When elements can fail, the pipeline carries the errors and the *consumer*
picks the policy — abort, gather, or skip:

```go
nums := catena.FromSlice(lines).MapErr(strconv.Atoi)

// pick one:
vals, err   := nums.Collect()          // stop at the first bad line
vals, errs  := nums.CollectAll()       // keep everything, report everything
vals        := nums.Ignore().Collect() // skip bad lines
```

Producers that own a resource open it lazily *inside* the pipeline, so an
unconsumed pipeline holds nothing and a downstream `Take(3)` still closes
the rows — proven against the real `database/sql` stack in
[bake_test.go](bake_test.go).

## The operators worth switching for

Not syntax sugar — these change the memory class of the computation.

| Use | Instead of | And it costs |
|---|---|---|
| `FoldBy` | `GroupBy` + fold per bucket | 2.97 MB → **1.2 KB** (bounded by keys, not elements) |
| `TopNBy(10, sel)` | `SortedDesc().Take(10)` | 4.1 MB → **1 KB**, 32× faster (bounded heap) |
| `DedupeBy` | `DistinctBy` on sorted input | unbounded seen-set → **O(1)** |

## Performance, before you choose

Measured three ways — a hand-written loop, the same pipeline hand-built
from raw `iter.Seq` closures, and catena (Apple M5 Max, Go 1.27, 100k
elements, [bench_baseline_test.go](bench_baseline_test.go)):

| Shape | Hand loop | Raw closures | catena |
|---|---|---|---|
| 4-stage pipeline (filter→map→filter→sum) | 39µs | 350µs | **262µs** |
| `Sum` | 107µs | 178µs | **70µs** |
| `Contains` | — | 116µs | **70µs** |

Two facts fall out of that. **catena adds essentially nothing on top of
Go's iterator mechanism** — on these three shapes it beats the raw-closure
baseline outright, and across the [full table](docs/05-performance.md) its
worst showing is 13% behind on a single-stage `Map`, where the baseline
inlines into its caller and a library cannot. So what you give up is
close to what `iter.Seq` itself costs. And
that cost is real: against a plain loop doing trivial arithmetic a
pipeline runs about 6.6× slower — roughly 2.2ns per element across the
whole four-stage chain, in closure calls the compiler cannot flatten. If
your loop body does anything beyond arithmetic — parses, allocates, touches a map or a socket
— the per-element work buries it and the two converge.

[The full benchmarks](docs/05-performance.md) show the code both ways, so
you can judge what the nanoseconds buy.

## The shape of the library

| Type | What it is |
|---|---|
| `Seq[T]` | lazy `iter.Seq[T]` with 84 methods — 37 chainable, 47 terminal; `range` works directly |
| `Seq2[K, V]` | the stdlib pair currency; a bridge back to `Seq`, not a peer |
| `Try[T]` | `iter.Seq2[T, error]`; error policy chosen by the consumer |
| `List[T]` | eager `[]T` with the mirrored operation set — generated, and conformance-checked to agree with `AsSeq()` |

Interop is one conversion and zero cost, in both directions:
`slices.Sorted(s.Seq())` to hand a pipeline to the stdlib,
`catena.From(maps.Keys(m))` to take one back. Operations that constrain the
element type (`Distinct`, `Sorted`, `Sum`, `Max`, …) are package functions,
because a method on `Seq[T any]` may require nothing of `T` — so are
`Chunked`, `ChunkedBy` and `Windowed`, which return `Seq[[]T]` and would be
an instantiation cycle as methods. The chain continues normally after
either kind.

## Documentation

**[Operator reference](docs/operators/README.md)** — all 183 operators
across 14 pages, each with its signature, its memory and termination
behaviour, and a worked example that `go test` runs and verifies.

Guides, in reading order:

1. [Getting started](docs/01-getting-started.md) — the five-minute tour
2. [Concepts](docs/02-concepts.md) — the four types, eager vs lazy, and the contract system
3. [Error handling](docs/03-error-handling.md) — `Try` and resource-owning producers
4. [Operator catalog](docs/04-operators.md) — the index, with memory and drain classes
5. [Performance](docs/05-performance.md) — the three-way benchmarks, honestly
6. [Releasing](docs/06-releasing.md) — how a release is cut and verified

Also: [runnable examples](examples/) · [API reference](https://pkg.go.dev/github.com/NerdMeNot/catena)
· [SPEC.md](SPEC.md), the full design — every decision, and what was rejected.

## Status

**`v1.1.0`** is the first supported release, and it freezes the API:
anything removed or changed incompatibly waits for a major version.
Earlier tags were withdrawn and are retracted in [go.mod](go.mod) — don't
use them. Changes are tracked in [CHANGELOG.md](CHANGELOG.md).

Licensed under [Apache-2.0](LICENSE).
