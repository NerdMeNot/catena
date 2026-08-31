# `catena` — complete design specification, v2

**Go 1.27+. Package `catena`.** Kotlin stdlib as inspiration, Rust's `Iterator` for what Kotlin lacks, LINQ for the relational edges, Go idiom as the arbiter of every dispute.

*Catena* — Latin for "chain." The library is the chain; the name is the promise.

This is v2 of the spec. v1 established the architecture; v2 closes every semantic gap found in two review passes so that implementation is transcription, not interpretation. Every operator now carries a complete contract: argument edges, empty-input behavior, error semantics, ordering guarantees, memory class, drain class, and statefulness. §0 records what changed. §13 is the implementation timeline.

---

## 0. Changes from v1

Recorded for traceability; the rest of the document is self-contained.

| Area | v1 | v2 |
|---|---|---|
| Package name | `seq` | `catena` |
| `StepBy` | violated §3 law | renamed `Step` |
| `Compact` | collided with `slices.Compact` (different meaning) | renamed `NonZero` |
| `List.TryAt` | collided with `Try[T]` type | renamed `List.Get` |
| `OrElse` | Optional-flavored misnomer | renamed `IfEmpty` |
| `JoinString` | root collision with `JoinBy` | renamed `JoinToString` |
| `Seq2.Collect` | duplicated `Unzip` | dropped; `Unzip` is the one way |
| `Try` invariant vs `Rows` example | contradictory | obligation split (§4.9); example rewritten with lazy acquisition (§4.2) |
| `Try` per-operator error semantics | unwritten | §4.9 rule set + per-operator notes |
| Argument edges (negative/zero counts, `Range` step) | unspecified | §4.10 family rules |
| Ordering, ties, collisions | unspecified | §4.11 |
| Float/NaN semantics | unspecified | §4.12 |
| Drain hazards | unmarked | law **L7**, three-way drain classification per terminal |
| L5 "no goroutines" vs `Zip`/`Equal` | contradiction (co-iteration requires `iter.Pull`) | L5 amended with documented `iter.Pull` carve-out |
| L1 stateful list | incomplete | completed; `Once()` named as the sanctioned exception |
| Conformance harness | `Seq→Seq` only, C1–C11 | five harness shapes, named fixtures, C1–C15 |
| `reflect` ban | unconditional | amended: permitted in `_test.go` only, for registration-completeness |
| List mirror | "generated or hand-mirrored" | codegen, specified in §10 |
| New operators | — | `ChunkedBy`, `Try.Errs`, `Try.UntilDone`, `Try.Pull`, `Seq2.Pull`, `Seq2.First`, `Seq.ForEachErr`, `Empty2`, `EmptyTry` |
| `Zip` experimental status | experimental | permanent (the §7.4 `Zip3` rejection depends on it) |
| Timeline | build order only | full phased timeline with exit criteria, §13 |
| Batch operators (`Chunked`, `ChunkedBy`, `Windowed`) | methods returning `Seq[[]T]` | **package functions** — a method on `Seq[T]` returning `Seq[[]T]` is an instantiation cycle (v2.1, found in implementation; §7.8) |
| §9.5 mechanism | `reflect` enumeration | `go/parser` enumeration — reflect cannot see uninstantiated generic methods (v2.1) |

---

## 1. Design axioms

Seven statements. Everything else is derived. If a proposed feature violates one, it doesn't go in.

1. **No `reflect`, no `any` in value position.** Not in a fast path, not in a fallback, not for convenience. `error` is the only interface that appears, and it was already one. *One amendment:* `reflect` is permitted inside `_test.go` files only (oracle comparisons via `reflect.DeepEqual`). It never ships in the library proper. Symbol enumeration for the completeness check (§9.5) uses `go/parser`, not reflect — reflect cannot see uninstantiated generic methods (§7.3).
2. **Zero abstraction the caller didn't ask for.** No hidden buffering, no hidden goroutines, no hidden allocation per element. Where an operation must buffer, it is named and documented as buffering.
3. **Laziness is a type, not a mode.** `Seq` is lazy, `List` is eager, and crossing between them is an explicit call. No operation silently changes evaluation strategy.
4. **Early termination is sacred.** Every operator propagates `yield`'s false return immediately. A single violation makes `Take(1)` on an infinite sequence hang forever.
5. **The type system carries the guarantee.** If it compiles, it does not panic on type grounds. Documented exceptions: `comparable` (§4.7) and construction-time argument panics (§4.10), which are contract violations, not type failures.
6. **Predictable names beat clever names.** The suffix law in §3 is binding. A reader who knows five operations can guess the sixth.
7. **Don't reimplement the standard library.** `slices` and `maps` exist. `Collect()` is the handoff point.

---

## 2. Type model

| Type | Definition | Passes | Infinite? | Random access |
|---|---|---|---|---|
| `Seq[T any]` | `iter.Seq[T]` | single (by contract) | yes | no |
| `List[T any]` | `[]T` | multi | no | yes |
| `Seq2[K, V any]` | `iter.Seq2[K, V]` | single | yes | no |
| `Try[T any]` | `iter.Seq2[T, error]` | single | yes | no |

```go
package catena

type Seq[T any]        iter.Seq[T]
type Seq2[K, V any]    iter.Seq2[K, V]
type Try[T any]        iter.Seq2[T, error]
type List[T any]       []T

type Numeric interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr |
    ~float32 | ~float64
}
type Integer interface {
    ~int | ~int8 | ~int16 | ~int32 | ~int64 |
    ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}
```

Still no numeric constraint in the standard library as of 1.27. Declare it once, export it, don't make callers import `x/exp`. `Numeric` deliberately excludes `complex64`/`complex128`: they are not ordered, half the aggregation surface (`Max`, `Average` comparisons) would be meaningless, and the remaining half (`Sum`, `Product`) doesn't justify a second constraint. Rejected, recorded in §7.7.

### 2.1 Why `Seq` must be a defined function type

`for v := range s` has to work directly, with no `.Seq()` call. Range-over-func accepts any type whose underlying type is `func(func(V) bool)`. A struct wrapper cannot be ranged over, so it is disqualified regardless of its other merits.

The cost of that decision, stated plainly: **no size hint.** Rust carries `size_hint` on every iterator so `collect` can preallocate. `Seq` cannot, so `Collect()` grows its slice. For sequences of known length, go through `List` or `slices.Collect` on a sized source. This is a real and permanent loss, and it's the price of `range` working.

The benefit is that conversion in every direction is free, `Seq` is nil-able (§4.6), and any third-party `iter.Seq` alias interoperates without a copy.

### 2.2 `Seq2` stays, `Pair` does not — resolved

`iter.Seq2` is the standard library's currency — `maps.All`, `slices.All`, `maps.Collect` all speak it. With `Seq2`, every stdlib boundary is a free type conversion. With `Pair`, every boundary needs a mapping stage: a closure and a call per element on the hottest path in the library. Rejected.

Mitigation for the surface cost: `Seq2` is deliberately a **bridge, not a peer**. It gets the operations needed to get back to `Seq[T]` quickly and nothing else. No `Seq2.GroupBy`, no `Seq2.SortedBy`, no `Seq2.FoldBy`. Call `.Keys()`, `.Values()`, or `.MapTo(...)` and continue in `Seq`.

### 2.3 `Try` semantics — resolved

`Try[T]` is `iter.Seq2[T, error]`, one error slot per element. Two policies are possible for what an error means, and the design does **not** choose between them — the consumer does:

- `Collect() ([]T, error)` — stop at the first error.
- `CollectAll() ([]T, []error)` — drain, gather everything.
- `Ignore() Seq[T]` — drop failed elements, continue.

Both policies are legitimate in the same codebase: a malformed line in a log scan should probably be skipped, a failed row scan should probably abort. Encoding the policy in the type would force one.

The zero-value invariant is an **obligation split** (this replaces v1's absolute phrasing, which its own example violated):

- **Producers SHOULD yield the zero `T` alongside a non-nil error.** All `catena` operators that generate errors (`MapErr`, `FilterErr`, `UntilDone`) do.
- **Consumers MUST NOT read `T` when `err != nil`,** because third-party producers may not zero it.

The full per-operator error semantics are in §4.9 and are uniform: one rule set, applied everywhere.

---

## 3. The naming law

Binding. A new operation that doesn't fit gets renamed, not exempted.

| Pattern | Meaning | Example |
|---|---|---|
| *(none)* | base operation | `Filter`, `Take`, `Fold`, `Step` |
| `-By[K]` | selector produces the key that drives the operation | `SortedBy`, `DistinctBy`, `GroupBy`, `MaxBy`, `FoldBy`, `ChunkedBy` |
| `-Of[K]` | selector produces a projection; **projections** come out | `SumOf`, `MaxOf`, `AverageOf` |
| `-With` | takes a comparator or equality function | `SortedWith`, `MaxWith`, `DistinctWith` |
| `-Indexed` | callback also receives the element index | `MapIndexed`, `FilterIndexed`, `FoldIndexed` |
| `-Not` | negated predicate | `FilterNot` |
| `-Err` | callback may fail; result is a `Try` | `MapErr`, `FilterErr`, `FoldErr`, `ForEachErr` |
| `-Where` | predicate narrows a base terminal | `CountWhere` |
| `-To` | crosses a type family (`Seq2`→`Seq`, seq→string) | `MapTo`, `JoinToString` |
| `To-` *(prefix)* | terminal that materialises into a named container | `ToList`, `ToChan`, `ToKeySet` |
| `-Desc` | descending order; the unsuffixed name is ascending | `SortedDesc`, `SortedByDesc` |
| `N` *(infix)* | the operation is bounded by a count argument | `TopN`, `TopNBy`, `BottomNBy`, `RepeatN` |
| `-While` | consumes or generates until a predicate turns false | `TakeWhile`, `DropWhile`, `FoldWhile`, `GenerateWhile` |
| `-Last` | the same operation anchored at the end | `TakeLast`, `DropLast`, `FindLast` |
| `-Map` *(fusion suffix)* | fuses a map into the base operation | `FilterMap`, `FlatMap`, `FindMap` |
| `-All` | the exhaustive variant of a short-circuiting terminal | `CollectAll` |
| `Is-` / `If-` *(prefix)* | predicate on the sequence / conditional substitution | `IsEmpty`, `IfEmpty` |
| `Until-` *(prefix)* | bounded by an external signal | `UntilDone` |
| `Non-` *(prefix)* | excludes a distinguished value | `NonZero` |

**Known exceptions**, recorded rather than hidden — the law is binding for
new operations, and these predate it or borrow a name whose recognition is
worth more than the consistency:

- `AssociateWith` takes a value-producing selector, not a comparator, so by
  the law's own vocabulary it is an `-Of`. The name is Kotlin's
  `associateWith`, and matching that is worth more than the row.
- `WithIndex` and `ZipWithNext` use `With` as a prefix meaning "paired
  with", unrelated to the `-With` suffix's "takes a comparator".
- `Non-` (`NonZero`) and `-Not` (`FilterNot`) both negate; each is the
  natural English for its own name.

The `-By` wording is deliberate: "the key that drives the operation," not "elements come out." `MaxBy(sel)` returns the **element** with the largest key; `FoldBy(key, ...)` returns **accumulators** per key; `TallyBy(sel)` returns **counts** per key. What comes out is whatever the base operation produces; the `-By` selector chooses the key it's driven by. The `-By` / `-Of` distinction is the one people get wrong: `MaxBy(sel)` returns the element, `MaxOf(sel)` returns the largest key. Both are wanted often enough to justify both.

`To` reads differently at each end, which is why both rows exist: as a suffix it names the family being crossed into (`MapTo` lands in `Seq`, `JoinToString` in a string), while as a prefix it names the container being built (`ToList`, `ToChan`). Suffix `-To` keeps you in a pipeline; prefix `To-` ends one.

Package-level functions take the unsuffixed name and carry the constraint: `catena.Distinct[T comparable]`, `catena.Max[T cmp.Ordered]`. So `Distinct` is always the direct form and `DistinctBy` is always the selector form, whichever side of the method boundary you're on.

An operation lands at package level for one of **three** reasons, not one: it constrains the element type (`Distinct`, `Sorted`, `Sum`, `Max`, `Union`); it returns `Seq[[]T]`, which as a method is an instantiation cycle (`Chunked`, `ChunkedBy`, `Windowed`, §7.8); or it constrains the receiver's *shape* rather than its element (`Flatten` needs `Seq[Seq[T]]`, `FlattenSlices` needs `Seq[[]T]`, `Unzip` needs a `Seq2`). `Cycle` and `Chain` are neither — they are placed with the constructor and combining families by choice, and would compile as methods.

**Cross-ecosystem collision rule** (new in v2): a `catena` name must not collide with a `slices`/`maps`/`strings` name *unless the semantics are identical*. This is why v1's `Compact` (drop zero values) became `NonZero` — `slices.Compact` means collapse consecutive duplicates, which `catena` calls `Dedupe`. Same name, adjacent ecosystem, different meaning is the worst naming bug available.

---

## 4. Contracts

The part that determines correctness. Every one of these has a line in the package doc.

### 4.1 Single-pass by contract, re-iterable by luck

`Seq[T]` is a function; calling it twice runs it twice. Whether that *works* depends entirely on the producer. `FromSlice` re-reads the slice and is fine. `FromChan` yields nothing the second time. A `sql.Rows` producer returns an error.

**The contract: treat every `Seq` as single-pass.** Do not consume one twice. The type system will not stop you and there is no formulation that would.

Two supports:

```go
func (s Seq[T]) Once() Seq[T]   // panics on second iteration — use during development
```

and the per-constructor re-iterability table in §6.1. `Once()` is the Kotlin `constrainOnce()` equivalent, and it is a debugging tool, not a default. Its consumed-flag is unsynchronized: under concurrent misuse (§4.5) it is best-effort, not a guarantee.

Two operators consume destructively in ways that deserve explicit warning:

- **`IsEmpty()` consumes one element.** On a single-pass source, that element is *gone* — the rest of the sequence no longer contains it. On re-iterable sources this is invisible; on single-use sources it is a data-loss bug. The doc comment says so.
- **`Seq2.Keys()` then `Seq2.Values()` on the same single-pass `Seq2` is a double-consume bug.** Use `Unzip` (buffers both) or restructure.

**Do not attempt automatic detection.** Any scheme that tracks consumption requires mutable state on the `Seq`, which requires a struct, which breaks `range`. Settled.

### 4.2 Resource ownership lives in the producer — lazily acquired

A `Seq[Row]` over `*sql.Rows` must close the rows. There is no `Close` on `iter.Seq` and adding one would break `range`. The pattern that works is **lazy acquisition**: the producer *opens* the resource inside the iteration closure, so a `Try` that is built but never consumed holds nothing.

```go
func Rows[T any](open func() (*sql.Rows, error), scan func(*sql.Rows) (T, error)) catena.Try[T] {
    return func(yield func(T, error) bool) {
        var zero T
        rows, err := open()
        if err != nil {
            yield(zero, err)            // acquisition failure is the first element
            return
        }
        defer rows.Close()              // runs on normal exit AND early termination
        for rows.Next() {
            v, err := scan(rows)
            if err != nil {
                v = zero                // producer obligation, §2.3
            }
            if !yield(v, err) {
                return                  // defer fires
            }
        }
        if err := rows.Err(); err != nil {
            yield(zero, err)
        }
    }
}
```

Two properties, both load-bearing:

1. **Never iterated ⇒ never opened.** v1's version took an already-open `*sql.Rows` and leaked it if the sequence was abandoned before consumption. Take an opener, not a resource.
2. **Early termination closes.** `yield` returning false returns from the producer, which runs its defers. This composes through arbitrarily many intermediate stages, because each stage returns as soon as its downstream `yield` returns false.

**The one hole: `Pull()`.** `iter.Pull` inverts control; if the caller never calls `stop`, the producer never returns and its defers never run. `Pull` transfers ownership. Document it in capitals:

```go
// Pull converts s to a pull-based iterator. THE CALLER MUST CALL stop,
// even if next has returned false, or resources held by s will leak.
func (s Seq[T]) Pull() (next func() (T, bool), stop func())
```

`ToChan` has the same property and takes a `context.Context` for exactly this reason.

### 4.3 Context belongs at the edges

No `WithContext` threaded through every stage. Cancellation enters at the producer (`FromChan(ctx, ch)`) or via an explicit gate:

```go
func (s Seq[T]) UntilDone(ctx context.Context) Try[T]   // yields (zero, ctx.Err()) and stops
func (t Try[T]) UntilDone(ctx context.Context) Try[T]   // same; Try is the I/O type — it needs this most
```

Threading a context through 40 operators would put a `select` in the per-element path of every stage to serve a case that only matters at I/O boundaries. `UntilDone` checks `ctx.Err()` (a cheap atomic load, no `select`) once per element in the single stage where the caller asked for it. Keep it at the edges.

### 4.4 Panics propagate; library panics are prefixed

If `yield` panics, the operator does not recover. If a user's selector panics, it propagates. No operator in this library calls `recover`. A pipeline is not a fault boundary and pretending otherwise hides bugs.

Panics the library itself raises (contract violations, §4.10) use the message form **`catena: <Op>: <reason>`** — e.g. `catena: Chunked: size must be > 0, got -3` — and are raised **at operator construction time**, not at first iteration, so the stack trace points at the caller's mistake, not at a range loop three files away.

**Nil callbacks are not defended.** `Filter(nil)` panics at first use with the runtime's nil-func panic. Checking every callback for nil at every construction is noise protecting against a bug no one writes twice; the doc states it and moves on.

### 4.5 Not safe for concurrent use

A single `Seq` consumed from two goroutines is a data race in the producer's state. Not detectable, not defended against. `ToChan` is the fan-out mechanism.

### 4.6 Nil is empty

`var s Seq[int]` is a nil func; ranging over it directly panics. **Every method and every package function treats a nil sequence as an empty sequence.** `nil.Filter(f).Collect()` returns nil. `catena.Distinct(nil)` is empty. Same for `Seq2`, `Try`, and `List` (where `[]T(nil)` already behaves).

Two members need explicit special-casing because their underlying stdlib calls would panic on nil:

- **`Pull()` on nil:** returns a `next` that immediately reports false and a no-op `stop`. (`iter.Pull(nil)` would panic on first `next`.)
- **`ToChan()` on nil:** returns an already-closed channel.

Enforced by conformance row C1, which runs every operator against a nil receiver *and* nil package-function arguments.

### 4.7 `comparable` does not mean panic-free

Since Go 1.20, ordinary interface types satisfy `comparable`. So:

```go
s := catena.Of[any](1, []int{2})
catena.Distinct(s)   // compiles; panics at runtime on the slice
```

The type system will not catch this. A hand-rolled constraint enumerating safe kinds would exclude user-defined comparable structs, so no formulation gets both. This is a documented exception to axiom 5. Say so in the doc comment of every `comparable`-constrained function.

### 4.8 Element copy cost

`Seq[T]` copies `T` at every stage boundary. For a 200-byte struct across a 6-stage pipeline that's 1.2KB of copying per element. Recommend `Seq[*T]` for large structs and say so; don't try to be clever about it.

### 4.9 `Try` error semantics — the uniform rule set

Five rules. Every `Try` operator follows them; per-operator notes in §6.6 only state which rules apply, never new ones.

- **R1 — Pass-through.** Intermediate operators never inspect errored elements: `Filter`'s predicate, `Map`'s function, `TakeWhile`'s predicate, `FlatMap`'s function are **not called** on an element whose `err != nil`. The errored element flows downstream unchanged. Only `Recover`, `WrapErr`, `OnError`, and the terminals engage with errors.
- **R2 — Positional counting.** The positional intermediates (`Take`, `Drop`) count **elements, errored or not**. `Take(n)` consumes at most n elements of the source — the property the termination reasoning in §4 depends on. The spelling for "n successes" is `.Ignore().Take(n)`. `Count` is a terminal, not a positional intermediate: it follows R5 and stops at the first error, returning the successes counted so far.
- **R3 — Predicates see successes only.** A consequence of R1 worth stating separately because of `TakeWhile`: an errored element passes through and does **not** terminate the sequence; only a successful element failing the predicate ends it.
- **R4 — Error generation zeroes.** Any `catena` operator that produces an error (`MapErr`/`FilterErr` when the callback fails, `UntilDone` on cancellation) yields `(zero, err)`, never `(v, err)`.
- **R5 — First-error termination for single-error terminals.** `Collect`, `Fold`, `Err`, `Count`, `ForEach` stop consuming at the first error and return it (alongside whatever partial result the signature carries). Only `CollectAll` and the intermediates continue past errors.

### 4.10 Argument and result-shape family rules

Stated once here; §6 repeats them only where an operator deviates.

**Count and size arguments.**

- Negative count → **panic at construction** (`catena:` prefix, §4.4): applies to `Take`, `Drop`, `TakeLast`, `DropLast`, `RepeatN`, `TopNBy`, `BottomNBy`, and the `Seq2`/`Try` variants. Fail fast, like `strings.Repeat`.
- Zero count is **valid and meaningful**: `Take(0)` = empty, `Drop(0)` = identity, `TakeLast(0)` = empty, `RepeatN(v, 0)` = empty, `TopNBy(0, sel)` = empty slice.
- Structural sizes must be positive → panic on `≤ 0`: `Chunked(n)`, `Windowed(size, step)` (both), `Step(n)`.
- `Range(start, stop, step)`: `step == 0` panics. Sign mismatch (`start < stop` with negative step, or the reverse) yields **empty** — Python semantics, so computed steps don't panic. **Termination is overflow-guarded**: the implementation must detect wraparound (compare-before-add or post-add wrap check) so `Range(math.MaxInt8-1, math.MaxInt8, int8(3))` terminates rather than looping forever. Unsigned `I` cannot step downward; the doc says so.
- `ElementAt(i)` with `i < 0` → `(zero, false)`, not a panic: it's in the comma-ok family, and `FindIndex`'s `-1` sentinel can flow into it; totality wins here.
- `List.At(i)` and `List.Slice(i, j)` panic exactly like the underlying index/slice expression — they *are* the underlying expression. `List.Get(i)` is the comma-ok form.

**Empty-input results.**

- Every `(T, bool)` / `(K, bool)` terminal returns `(zero, false)` on empty input: `First`, `Last`, `Single`, `Find*`, `Reduce`, `Max*`, `Min*`, `Average*`, `MinMaxOf` (both zeros, false).
- `Sum` on empty = `0`. **`Product` on empty = `1`** — the multiplicative identity, documented loudly because it surprises people.
- `Average`/`AverageOf` accumulate in `float64` (running sum + count); precision for large `int64` inputs is explicitly **not** guaranteed (no Kahan summation — stated as a non-guarantee).
- `Single()` returns true iff exactly one element; it stops consuming immediately upon seeing a second. It can still hang on an infinite source that yields one element and then stalls — that is the source's nature, not the operator's.

**Nil-for-empty.** Every slice-returning operation returns `nil` for an empty result — matching `slices.Collect`. Applies to `Collect`, `GroupBy` (never creates empty buckets anyway), `Partition` (either side), `TopNBy`, `CollectAll` (both slices; a fully-successful drain returns a nil error slice), `Unzip`. Callers who need non-nil empty slices are holding it wrong; `len` and `range` don't care.

**`Try.Collect` on error** returns the partial `[]T` gathered before the error, plus the error. Callers who want all-or-nothing discard the slice when `err != nil`.

### 4.11 Ordering, ties, and collisions

- **Encounter order is preserved** by every operator that doesn't sort: `Filter`, `Map`, `Distinct*`, `Union`, `Intersect`, `Except`, `Partition` (within each side), `GroupBy` buckets (within each bucket).
- **Map-returning terminals** (`GroupBy`, `FoldBy`, `TallyBy`, `IndexBy`, `Associate`, `AssociateWith`, `Tally`, `ToKeySet`, `CollectMap`) return Go maps: **iteration order is undefined**, stated once in the package doc, not per operator.
- **Key collisions: last wins** for `IndexBy`, `Associate`, `AssociateWith`, `CollectMap` — plain map assignment semantics, matching `maps.Collect` and Kotlin's `associateBy`.
- **First wins** for `Distinct`/`DistinctBy` (first occurrence survives, in encounter position) and for `Max*`/`Min*` ties (the earliest maximal element is returned, matching Kotlin).
- **Set operations are set-semantics**: `Union`, `Intersect`, `Except` dedupe their output, yield in encounter order, left operand first (`Union`). All three carry unbounded seen-set memory marks (§6.4).
- **`TopNBy`/`BottomNBy` output is sorted** (descending for Top, ascending for Bottom) and **stable**: the heap tracks insertion index and breaks key-ties by earlier index, preserving L6 across the one operator where a naive heap would silently violate it.
- **`Generate`/`GenerateWhile` yield the seed first**, always — `GenerateWhile`'s continue-check applies to *produced* values, so the seed is unconditional.
- **`Cycle` of an empty sequence terminates** (yields nothing). Without this guard the operator hot-spins forever — the classic `Cycle` bug, specified away.

### 4.12 Float semantics

- `Max`/`Min`/`MaxOf`/`MinOf`/`MinMaxOf`/`MaxBy`/`MinBy`/`TopNBy`/`BottomNBy` all use **`cmp.Compare` ordering**, under which NaN orders before every other value: `Max` over `{1, NaN}` is 1; `Min` over `{1, NaN}` is NaN. One convention across the aggregation surface — no operator uses raw `<`/`>` where NaN would make them disagree with the sorts.
- `Sorted*`/`SortedBy` use `cmp.Compare`/`cmp.Less`: NaN sorts first. Same convention, for free.
- `Sum`/`Product`/`Average` use naive accumulation; NaN and ±Inf propagate per IEEE 754. No compensation, stated as a non-guarantee (§4.10).

---

## 5. Implementation laws

Not style preferences. Each corresponds to a bug this class of library reliably ships with.

**L1 — Operator state is initialized inside the returned closure, never in the enclosing function.**

```go
// WRONG — n is captured once; second iteration takes zero elements
func (s Seq[T]) Take(n int) Seq[T] {
    return func(yield func(T) bool) {
        for v := range iter.Seq[T](s) {
            if n <= 0 { return }
            n--
            if !yield(v) { return }
        }
    }
}

// RIGHT — state is per-iteration
func (s Seq[T]) Take(n int) Seq[T] {
    if n < 0 { panic("catena: Take: negative count") }
    return func(yield func(T) bool) {
        left := n
        for v := range iter.Seq[T](s) {
            if left <= 0 { return }
            left--
            if !yield(v) { return }
        }
    }
}
```

This is the number one bug in every library of this shape. It only manifests on re-iteration, which test suites usually don't do (ours does — C2). The **complete** stateful list, each of which must pass C2:

`Take` · `Drop` · `TakeWhile` · `DropWhile` · `TakeLast` · `DropLast` · `Step` · `Distinct` · `DistinctBy` · `DistinctWith` · `Dedupe` · `DedupeBy` · `Chunked` · `ChunkedBy` · `Windowed` · `WithIndex` · `Scan` · `ZipWithNext` · `Intersperse` · `IfEmpty` (yielded-anything flag) · `Cycle` (replay buffer) · `Zip` (pull handle) · `MapIndexed` · `FilterIndexed` · `FoldIndexed` · `ForEachIndexed` (index counters — the index-continues-across-reiteration bug) · every `Try`/`Seq2` counterpart of the above.

**The sanctioned exception: `Once()`.** Its consumed-flag must live in the *enclosing* closure — surviving re-iteration is its entire purpose. It is registered in the harness with the C2 row explicitly inverted (second iteration must panic).

**L2 — Return immediately when `yield` returns false, and never call `yield` again after it does.** The rangefunc runtime panics on the latter for `range` statements, but hand-composed pipelines can violate it in ways the runtime won't catch. Conformance row C6.

**L3 — Never retain a slice passed to `yield`.** `Chunked`, `ChunkedBy`, and `Windowed` allocate a fresh slice per emission — including overlapping `Windowed` windows that share elements. No buffer reuse, no "unsafe fast variant." The allocation is negligible next to whatever the caller does with a batch, and reuse corrupts data for anyone who retains it.

**L4 — Buffering operators state their bound.** Every operator doc declares its memory class; the consolidated table is §6.8. Classes: **O(1)** (streaming), **O(k)** (bounded by an argument), **O(keys)** (bounded by distinct-key count — `FoldBy`, `Distinct`), **O(all)** (buffers the entire input — `Sorted*`, `Reversed`).

**L5 — No goroutines, with one named carve-out.** No operator spawns a goroutine, except:

- `ToChan`, which is explicitly a concurrency primitive and takes a context.
- **`Zip` and `Equal`, which use `iter.Pull` internally** — there is no other way to co-iterate two push sequences, and `iter.Pull` runs the producer in a runtime coroutine. Both call `defer stop()` on every exit path, so the pulled producer's defers always run (§4.2 composes). This is a documented mechanism, not a leak: the coroutine cannot outlive the operator.

An operator that spawns outside this list is a bug.

**L6 — Sorts are stable.** `SortedBy`, `SortedWith`, `Sorted`, `SortedDesc` use `slices.SortStableFunc`. Selector-based sorting is almost always a secondary sort, and an unstable one silently destroys prior ordering. `TopNBy` preserves stability via index-tracked tie-breaking (§4.11). If benchmarks ever demand it, add `SortedByUnstable` — don't make the default the fast-and-surprising one.

**L7 — Drain classes are declared.** Every terminal is classified, in its doc comment and in §6.8:

- **∞-safe** — consumes a bounded prefix regardless of input: `First`, `Take`-then-anything, `ElementAt`, `IsEmpty`.
- **conditional** — terminates only if the answer arrives: `Any`/`Find`/`FindMap`/`FindIndex` (hang if no match on infinite input), `All`/`None` (hang if no counterexample), `Single` (hangs if no second element arrives).
- **⚠ full-drain** — consumes everything; **hangs on infinite input**: `Last`, `FindLast`, `Count`, `CountWhere`, `Collect`, `Fold*`, `Reduce`, all grouping and aggregation terminals, `Reversed`, `Sorted*`, `TakeLast`, `DropLast` (drains to emit), `Unzip`, `Equal` (up to first difference — full drain when equal), `Drain`.

---

## 6. API

Family rules from §4.9–§4.12 apply everywhere and are not repeated per operator; the tables below state only each operator's **class markers** and anything operator-specific.

**Legend** — `mem`: O(1) streaming · O(k) arg-bounded · O(keys) distinct-key-bounded · **all** buffers entire input. `[S]`: stateful, on the L1 list, gated by C2. `⚠∞`: full-drain, hangs on infinite input (L7). `◐`: conditional drain. Unmarked terminals are ∞-safe.

### 6.1 Constructors

```go
func Of[T any](vals ...T) Seq[T]
func From[T any](seq func(func(T) bool)) Seq[T]              // literal type: accepts any alias
func From2[K, V any](seq func(func(K, V) bool)) Seq2[K, V]
func FromErrs[T any](seq func(func(T, error) bool)) Try[T]
func FromSlice[T any](s []T) Seq[T]
func FromMap[K comparable, V any](m map[K]V) Seq2[K, V]
func FromChan[T any](ctx context.Context, ch <-chan T) Seq[T]

func Empty[T any]() Seq[T]
func Empty2[K, V any]() Seq2[K, V]
func EmptyTry[T any]() Try[T]
func Once1[T any](v T) Seq[T]
func Repeat[T any](v T) Seq[T]                                // ∞
func RepeatN[T any](v T, n int) Seq[T]                        // n<0 panics
func Generate[T any](seed T, next func(T) T) Seq[T]           // ∞; seed yielded first
func GenerateWhile[T any](seed T, next func(T) (T, bool)) Seq[T]  // seed yielded unconditionally
func Range[I Integer](start, stop, step I) Seq[I]             // §4.10: step==0 panics; sign mismatch → empty; overflow-guarded
func Cycle[T any](s Seq[T]) Seq[T]                            // ∞; mem: all (replay buffer); empty s → empty [S]

func Self[T any](v T) T                                        // identity selector
```

`From` takes the literal `func(func(T) bool)` rather than `iter.Seq[T]` so it accepts `iter.Seq`, `Seq`, and any third-party alias with no conversion at the call site.

`Once1` rather than `Once` — `Once()` is the single-use guard method in §4.1 and the collision would be miserable.

`FromChan` starts no goroutine; it receives inside the iteration closure with a two-case `select` on `ctx.Done()` and the channel. Never consumed ⇒ never receives.

**Re-iterability table** (normative for the docs):

| Re-iterable | Single-use |
|---|---|
| `Of`, `FromSlice`, `FromMap`, `Empty*`, `Once1`, `Repeat`, `RepeatN`, `Generate*` (iff `next` is pure), `Range` | `From`/`From2`/`FromErrs` (depends on source), `FromChan`, anything over I/O |

`Cycle` is re-iterable iff its source's first pass is (it replays from its buffer thereafter).

### 6.2 `Seq[T]` — intermediate

**No new type parameter:**

```go
func (s Seq[T]) Filter(pred func(T) bool) Seq[T]
func (s Seq[T]) FilterNot(pred func(T) bool) Seq[T]
func (s Seq[T]) FilterIndexed(pred func(int, T) bool) Seq[T]      // [S]
func (s Seq[T]) Take(n int) Seq[T]                                // [S]
func (s Seq[T]) TakeWhile(pred func(T) bool) Seq[T]               // [S]
func (s Seq[T]) TakeLast(n int) Seq[T]                            // [S] mem O(k); ⚠∞ drains to emit
func (s Seq[T]) Drop(n int) Seq[T]                                // [S]
func (s Seq[T]) DropWhile(pred func(T) bool) Seq[T]               // [S]
func (s Seq[T]) DropLast(n int) Seq[T]                            // [S] mem O(k), emits with n-element lag
func (s Seq[T]) Step(n int) Seq[T]                                // [S] every nth, first always; n≤0 panics
func (s Seq[T]) OnEach(f func(T)) Seq[T]                          // side effects, passthrough
func (s Seq[T]) Concat(others ...Seq[T]) Seq[T]
func (s Seq[T]) Append(vals ...T) Seq[T]
func (s Seq[T]) Prepend(vals ...T) Seq[T]
func (s Seq[T]) Intersperse(sep T) Seq[T]                         // [S]
func (s Seq[T]) IfEmpty(defaults ...T) Seq[T]                     // [S] if s yields nothing, yield defaults
func (s Seq[T]) WithIndex() Seq2[int, T]                          // [S] index from 0
func (s Seq[T]) ZipWithNext() Seq2[T, T]                          // [S] empty/single input → empty
func (s Seq[T]) SortedWith(cmp func(a, b T) int) Seq[T]           // mem all; stable; ⚠∞
func (s Seq[T]) DistinctWith(eq func(a, b T) bool) Seq[T]         // [S] mem O(distinct), O(n²) compares — small inputs only
func (s Seq[T]) Reversed() Seq[T]                                 // mem all; ⚠∞
func (s Seq[T]) Once() Seq[T]                                     // §4.1 guard; sanctioned L1 exception
func (s Seq[T]) UntilDone(ctx context.Context) Try[T]             // §4.3
```

**New type parameter:**

```go
func (s Seq[T]) Map[U any](f func(T) U) Seq[U]
func (s Seq[T]) MapIndexed[U any](f func(int, T) U) Seq[U]        // [S]
func (s Seq[T]) FilterMap[U any](f func(T) (U, bool)) Seq[U]      // fused; Rust's filter_map
func (s Seq[T]) FlatMap[U any](f func(T) Seq[U]) Seq[U]
func (s Seq[T]) FlatMapSlice[U any](f func(T) []U) Seq[U]
func (s Seq[T]) Scan[A any](init A, f func(A, T) A) Seq[A]        // [S] Kotlin runningFold; init NOT yielded, first output f(init, e0)
func (s Seq[T]) Zip[U any](other Seq[U]) Seq2[T, U]               // [S] L5 carve-out; see below
func (s Seq[T]) MapErr[U any](f func(T) (U, error)) Try[U]        // R4: error → (zero, err)
func (s Seq[T]) FilterErr(pred func(T) (bool, error)) Try[T]      // R4
func (s Seq[T]) DistinctBy[K comparable](sel func(T) K) Seq[T]    // [S] mem O(keys); first occurrence wins
func (s Seq[T]) DedupeBy[K comparable](sel func(T) K) Seq[T]      // [S] consecutive only, O(1)
func (s Seq[T]) SortedBy[K cmp.Ordered](sel func(T) K) Seq[T]     // mem all; stable; ⚠∞
func (s Seq[T]) SortedByDesc[K cmp.Ordered](sel func(T) K) Seq[T] // mem all; stable; ⚠∞
func (s Seq[T]) JoinBy[U any, K comparable, R any](               // EXPERIMENTAL; see below
    other Seq[U],
    leftKey func(T) K, rightKey func(U) K,
    combine func(T, U) R,
) Seq[R]
```

**`Zip` contract.** The receiver drives (consumed by `range`); `other` is consumed through `iter.Pull` with `defer stop()` on every exit (L5 carve-out). Stops at the shorter side. Consumption on single-pass sources, stated exactly: `other` is pulled once per emitted pair — never over-consumed; the receiver is consumed one element **past** the pair count when `other` is shorter (the unpaired element is discarded). Permanent API — the §7.4 `Zip3` rejection depends on it.

**`JoinBy` contract.** Inner join: unmatched elements on either side are dropped. The right side is **fully buffered into `map[K][]U` before the first emission** (⚠ mem O(right); first-element latency is the full right drain). Output order: left encounter order; within one left element's matches, right encounter order. Duplicate keys produce the cross product per key. *Graduated from experimental after the bake pipelines:* benchmark overhead vs a hand-written hash join is ~1.5× (one ordinary iterator stage, no pathology), and it composed cleanly in the bake pipeline (JoinBy → FoldBy).

**`Scan` contract.** `init` is not yielded (Rust `scan`, not Kotlin `runningFold`'s include-initial): `Of(1,2).Scan(0, add)` yields `1, 3`. Callers wanting the initial value prepend it.

`Flatten` is `FlatMap(Self)` — on `Seq[Seq[int]]`, `T` infers to `Seq[int]` and `U` to `int`. Fully typed. A bare `.Flatten()` is not expressible (§7.2).

### 6.3 `Seq[T]` — terminal

```go
// Collection
func (s Seq[T]) Collect() []T                                  // ⚠∞; nil for empty
func (s Seq[T]) ToList() List[T]                               // ⚠∞
func (s Seq[T]) Seq() iter.Seq[T]
func (s Seq[T]) Pull() (next func() (T, bool), stop func())    // CALLER MUST CALL stop; nil-safe §4.6
func (s Seq[T]) ToChan(ctx context.Context) <-chan T           // unbuffered; goroutine starts at call; closed on completion AND ctx cancel
func (s Seq[T]) ForEach(f func(T))                             // ⚠∞
func (s Seq[T]) ForEachIndexed(f func(int, T))                 // ⚠∞
func (s Seq[T]) ForEachErr(f func(T) error) error              // stops at first non-nil f return, returns it; ◐
func (s Seq[T]) Drain()                                        // ⚠∞

// Search
func (s Seq[T]) First() (T, bool)
func (s Seq[T]) Last() (T, bool)                               // ⚠∞
func (s Seq[T]) Single() (T, bool)                             // ◐ stops at second element
func (s Seq[T]) ElementAt(i int) (T, bool)                     // i<0 → (zero,false)
func (s Seq[T]) Find(pred func(T) bool) (T, bool)              // ◐
func (s Seq[T]) FindLast(pred func(T) bool) (T, bool)          // ⚠∞
func (s Seq[T]) FindIndex(pred func(T) bool) int               // ◐; -1 if none
func (s Seq[T]) FindMap[U any](f func(T) (U, bool)) (U, bool)  // ◐

// Predicate
func (s Seq[T]) Any(pred func(T) bool) bool                    // ◐
func (s Seq[T]) All(pred func(T) bool) bool                    // ◐
func (s Seq[T]) None(pred func(T) bool) bool                   // ◐
func (s Seq[T]) Count() int                                    // ⚠∞
func (s Seq[T]) CountWhere(pred func(T) bool) int              // ⚠∞
func (s Seq[T]) IsEmpty() bool                                 // consumes one element — §4.1 warning

// Fold
func (s Seq[T]) Fold[A any](init A, f func(A, T) A) A                          // ⚠∞
func (s Seq[T]) FoldIndexed[A any](init A, f func(int, A, T) A) A              // ⚠∞
func (s Seq[T]) FoldWhile[A any](init A, f func(A, T) (A, bool)) A             // ◐ false stops, that result IS included
func (s Seq[T]) FoldErr[A any](init A, f func(A, T) (A, error)) (A, error)     // ◐ stops at first error, acc-so-far + err
func (s Seq[T]) FoldBy[K comparable, A any](                                    // ⚠∞; mem O(keys)
    key func(T) K, init func(K) A, f func(A, T) A,
) map[K]A
func (s Seq[T]) Reduce(f func(T, T) T) (T, bool)               // ⚠∞

// Grouping                                                     (all ⚠∞; map order undefined §4.11)
func (s Seq[T]) GroupBy[K comparable](sel func(T) K) map[K][]T     // buckets in encounter order
func (s Seq[T]) IndexBy[K comparable](sel func(T) K) map[K]T       // last wins
func (s Seq[T]) TallyBy[K comparable](sel func(T) K) map[K]int
func (s Seq[T]) Associate[K comparable, V any](f func(T) (K, V)) map[K]V   // last wins
func (s Seq[T]) Partition(pred func(T) bool) (yes, no []T)         // encounter order both sides

// Aggregation                                                  (all ⚠∞; NaN per §4.12; empty per §4.10)
func (s Seq[T]) MaxBy[K cmp.Ordered](sel func(T) K) (T, bool)      // ties: first wins
func (s Seq[T]) MinBy[K cmp.Ordered](sel func(T) K) (T, bool)
func (s Seq[T]) MaxOf[K cmp.Ordered](sel func(T) K) (K, bool)
func (s Seq[T]) MinOf[K cmp.Ordered](sel func(T) K) (K, bool)
func (s Seq[T]) MinMaxOf[K cmp.Ordered](sel func(T) K) (min, max K, ok bool)   // one pass
func (s Seq[T]) MaxWith(cmp func(a, b T) int) (T, bool)
func (s Seq[T]) MinWith(cmp func(a, b T) int) (T, bool)
func (s Seq[T]) TopNBy[K cmp.Ordered](n int, sel func(T) K) []T    // mem O(k); output sorted desc; stable §4.11
func (s Seq[T]) BottomNBy[K cmp.Ordered](n int, sel func(T) K) []T // mem O(k); output sorted asc; stable
func (s Seq[T]) SumOf[N Numeric](sel func(T) N) N                  // wraps like +
func (s Seq[T]) ProductOf[N Numeric](sel func(T) N) N              // empty → 1
func (s Seq[T]) AverageOf[N Numeric](sel func(T) N) (float64, bool)
func (s Seq[T]) JoinToString(sep string, sel func(T) string) string
```

**`FoldBy` is the operation that justifies the library.** Two fresh type parameters on one method, streaming per-key aggregation with no intermediate `map[K][]T`. Memory is O(distinct keys), not O(elements) — worth stating because "bounded by keys" is the difference between safe and OOM on a high-cardinality stream.

**`TopNBy` is the second.** `SortedBy(sel).Take(n)` buffers the entire sequence; `TopNBy` holds a bounded heap of n. Neither Kotlin nor Java's Stream offers it.

### 6.4 Package functions — constraint-bound

Cannot be methods (§7.1). Fully typed, no selector closure. All nil-safe (§4.6).

```go
func Distinct[T comparable](s Seq[T]) Seq[T]                  // [S] mem O(keys); first wins
func Dedupe[T comparable](s Seq[T]) Seq[T]                    // [S] consecutive, O(1)
func Sorted[T cmp.Ordered](s Seq[T]) Seq[T]                   // mem all; stable; ⚠∞
func SortedDesc[T cmp.Ordered](s Seq[T]) Seq[T]               // mem all; stable; ⚠∞
func Sum[T Numeric](s Seq[T]) T                               // ⚠∞; empty → 0
func Product[T Numeric](s Seq[T]) T                           // ⚠∞; empty → 1
func Average[T Numeric](s Seq[T]) (float64, bool)             // ⚠∞
func Max[T cmp.Ordered](s Seq[T]) (T, bool)                   // ⚠∞; cmp.Compare ordering
func Min[T cmp.Ordered](s Seq[T]) (T, bool)                   // ⚠∞
func MinMax[T cmp.Ordered](s Seq[T]) (min, max T, ok bool)    // ⚠∞; one pass
func TopN[T cmp.Ordered](s Seq[T], n int) []T                 // ⚠∞; mem O(k); sorted desc; stable
func Contains[T comparable](s Seq[T], v T) bool               // ◐
func IndexOf[T comparable](s Seq[T], v T) int                 // ◐; -1 if none
func NonZero[T comparable](s Seq[T]) Seq[T]                   // drop zero values (v1 "Compact", renamed §3)
func Chunked[T any](s Seq[T], n int) Seq[[]T]                 // [S] last chunk may be partial; n≤0 panics; L3; §7.8
func ChunkedBy[T any, K comparable](s Seq[T], sel func(T) K) Seq[[]T]  // [S] runs of equal keys; mem O(run); L3; §7.8
func Windowed[T any](s Seq[T], size, step int) Seq[[]T]       // [S] mem O(size); full windows only; step>size samples; ≤0 panics; L3; §7.8
func ToKeySet[T comparable](s Seq[T]) map[T]struct{}             // ⚠∞
func Tally[T comparable](s Seq[T]) map[T]int                  // ⚠∞
func AssociateWith[T comparable, V any](s Seq[T], f func(T) V) map[T]V   // ⚠∞; last wins
func Equal[T comparable](a, b Seq[T]) bool                    // consumes both to first difference (fully when equal); L5 carve-out
func Union[T comparable](a, b Seq[T]) Seq[T]                  // [S] set semantics; mem O(keys) — unbounded seen-set
func Intersect[T comparable](a, b Seq[T]) Seq[T]              // [S] mem O(b)+O(keys); b fully buffered first
func Except[T comparable](a, b Seq[T]) Seq[T]                 // [S] mem O(b)+O(keys); b fully buffered first
func Flatten[T any](s Seq[Seq[T]]) Seq[T]
func FlattenSlices[T any](s Seq[[]T]) Seq[T]
func Chain[T any](seqs ...Seq[T]) Seq[T]
func Unzip[K, V any](s Seq2[K, V]) ([]K, []V)                 // ⚠∞; mem all (both sides)
func CollectMap[K comparable, V any](s Seq2[K, V]) map[K]V    // ⚠∞; last wins
func Join(s Seq[string], sep string) string                   // ⚠∞
```

**`ChunkedBy` contract.** Splits into runs of equal keys: a chunk closes when `sel` changes value (consecutive comparison — the chunking cousin of `DedupeBy`). Memory is bounded by the longest run. `[a,a,b,a] → [a,a],[b],[a]`. This is the operator behind "group this sorted log by request id without buffering the file."

Set-operation semantics, stated once: output is deduplicated, in encounter order. `Union` yields a's distinct elements then b's not-yet-seen elements. `Intersect` yields a's distinct elements that occur in b. `Except` yields a's distinct elements that do not occur in b. All three maintain a seen-set (⚠ unbounded in distinct values); `Intersect`/`Except` additionally buffer b entirely before a is consumed.

### 6.5 `Seq2[K, V]` — bridge only

```go
func (s Seq2[K, V]) Filter(pred func(K, V) bool) Seq2[K, V]
func (s Seq2[K, V]) FilterNot(pred func(K, V) bool) Seq2[K, V]
func (s Seq2[K, V]) Map[K2, V2 any](f func(K, V) (K2, V2)) Seq2[K2, V2]
func (s Seq2[K, V]) MapValues[V2 any](f func(K, V) V2) Seq2[K, V2]   // f sees both K and V (Kotlin-consistent)
func (s Seq2[K, V]) MapTo[U any](f func(K, V) U) Seq[U]              // the intended exit to Seq
func (s Seq2[K, V]) Take(n int) Seq2[K, V]                           // [S]
func (s Seq2[K, V]) Drop(n int) Seq2[K, V]                           // [S]
func (s Seq2[K, V]) Keys() Seq[K]                                    // §4.1: Keys+Values on one single-pass source = double consume
func (s Seq2[K, V]) Values() Seq[V]
func (s Seq2[K, V]) Swap() Seq2[V, K]
func (s Seq2[K, V]) Fold[A any](init A, f func(A, K, V) A) A         // ⚠∞
func (s Seq2[K, V]) ForEach(f func(K, V))                            // ⚠∞
func (s Seq2[K, V]) Any(pred func(K, V) bool) bool                   // ◐
func (s Seq2[K, V]) All(pred func(K, V) bool) bool                   // ◐
func (s Seq2[K, V]) Count() int                                      // ⚠∞
func (s Seq2[K, V]) First() (K, V, bool)
func (s Seq2[K, V]) Seq2() iter.Seq2[K, V]
func (s Seq2[K, V]) Pull() (next func() (K, V, bool), stop func())   // iter.Pull2; CALLER MUST CALL stop; nil-safe
```

Deliberately absent, stated so nobody goes looking:

- **`ToMap`** — needs `K comparable` on the receiver (§7.1). Use `catena.CollectMap`.
- **`Collect() ([]K, []V)`** — was in v1; dropped. `catena.Unzip` is the one way, and `Collect` returning two slices didn't mean what `Collect` means everywhere else.
- **`MapKeys`** — bridge philosophy (§2.2): `Seq2` gets you back to `Seq`, it is not a place to live. `Swap().MapValues(f).Swap()` exists for the rare need; `Map` covers the rest.

### 6.6 `Try[T]`

Rules R1–R5 (§4.9) govern; notes below only bind rules to signatures.

```go
// Intermediate
func (t Try[T]) Map[U any](f func(T) U) Try[U]                // R1
func (t Try[T]) MapErr[U any](f func(T) (U, error)) Try[U]    // R1, R4
func (t Try[T]) FlatMap[U any](f func(T) Try[U]) Try[U]       // R1: errored input passes through un-mapped; inner errors flow in order
func (t Try[T]) Filter(pred func(T) bool) Try[T]              // R1
func (t Try[T]) FilterErr(pred func(T) (bool, error)) Try[T]  // R1, R4
func (t Try[T]) Take(n int) Try[T]                            // [S] R2: errored elements count
func (t Try[T]) TakeWhile(pred func(T) bool) Try[T]           // [S] R3: errors pass through, don't terminate
func (t Try[T]) Drop(n int) Try[T]                            // [S] R2
func (t Try[T]) OnEach(f func(T)) Try[T]                      // R1: successes only
func (t Try[T]) OnError(f func(error)) Try[T]                 // errors only; logging hook
func (t Try[T]) Recover(f func(error) (T, bool)) Try[T]       // true → (v, nil); false → error unchanged
func (t Try[T]) WrapErr(f func(error) error) Try[T]           // add context; nil return keeps ORIGINAL error
func (t Try[T]) UntilDone(ctx context.Context) Try[T]         // R4; §4.3

// Terminal
func (t Try[T]) Collect() ([]T, error)                        // R5: partial slice + first error
func (t Try[T]) CollectAll() ([]T, []error)                   // ⚠∞ drains; positional correspondence between slices is lost
func (t Try[T]) Fold[A any](init A, f func(A, T) A) (A, error) // R5: acc-so-far + first error
func (t Try[T]) ForEach(f func(T) error) error                // R5: stops at first of element-error OR f-error
func (t Try[T]) Ignore() Seq[T]                               // successes only
func (t Try[T]) Errs() Seq[error]                             // errors only — the dual of Ignore
func (t Try[T]) Must() Seq[T]                                 // panic(err) on first error — recover() receives the error value
func (t Try[T]) Err() error                                   // R5: stops at first error, returns it; nil if drained clean
func (t Try[T]) Count() (int, error)                          // R5: successes counted up to first error
func (t Try[T]) Pull() (next func() (T, error, bool), stop func())  // iter.Pull2; CALLER MUST CALL stop
func (t Try[T]) Seq2() iter.Seq2[T, error]
```

`WrapErr` earns its place: a scan pipeline five stages deep produces errors with no positional context, and `fmt.Errorf("row %d: %w", ...)` at the right stage is the difference between a debuggable error and a useless one. A `WrapErr` callback returning nil is a caller bug; the operator keeps the original error rather than converting a failure into a zero-value success.

`Errs` completes the fork: `t.Ignore()` and `t.Errs()` split one `Try` into its two streams — but they are two consumptions (§4.1); on a single-pass source use one or the other, or `CollectAll`.

### 6.7 `List[T]`

`List[T]` is `[]T`, so `[]T(l)` unwraps with no copy and no method is needed for it.

**List-only:**

```go
func (l List[T]) Len() int                                     // O(1)
func (l List[T]) At(i int) T                                   // panics like l[i]
func (l List[T]) Get(i int) (T, bool)                          // comma-ok; renamed from v1 TryAt (§3 collision with Try[T])
func (l List[T]) Slice(i, j int) List[T]                       // panics like l[i:j]; shares backing array (it IS l[i:j])
func (l List[T]) Clone() List[T]                               // shallow, fresh backing
func (l List[T]) AsSeq() Seq[T]
func (l List[T]) FoldRight[A any](init A, f func(T, A) A) A    // here only, §7.5
func (l List[T]) Append(vals ...T) List[T]                     // ALWAYS fresh backing array — never aliases l
```

**`Append` never aliases.** The built-in `append` may share the receiver's backing array; `List.Append` always allocates fresh (`O(len+k)`), because every other `List` transform returns freshly-allocated results and one aliasing exception would be a trap. Callers who want the built-in's amortized behavior use the built-in — `List` is a `[]T`. `Slice` is the documented opposite: it *is* the slice expression, aliasing included.

**Mirror rule:** every intermediate and terminal operation on `Seq[T]` exists on `List[T]` with the same name and the same suffix law, returning `List[U]` where `Seq` returns `Seq[U]`, and identical types elsewhere. The mirror is **generated** — §10 — and bound by conformance row C15: `l.Op(x)` ≡ `l.AsSeq().Op(x)` (collected) for every mirrored operator. Differences, all mechanical:

- Preallocates exactly: `Map` does `make([]U, 0, len(l))`, no growth. Every chained stage allocates one full slice — stated in the type doc; Kotlin's `List` operations behave identically and surprise people anyway.
- `Len`, `At`, `Last`, `Reversed`, `ElementAt` are O(1)/O(n) direct, not a drain.
- Multi-pass safe. No `Once()`, no re-iteration hazard, no `[S]` anywhere.
- No infinite constructors (`Repeat`, `Generate`, `Cycle` are `Seq`-only), no `Pull`, no `UntilDone`, no `ToChan`, no drain/buffer marks (everything is already a buffer).

Choosing between them: `Seq` for I/O, channels, large scans, anything where short-circuiting or unbounded length matters. `List` for small in-memory collections touched more than once. Cross explicitly — the way Kotlin makes you write `asSequence()`.

### 6.8 Consolidated contract tables

**Memory classes** (L4) — operators not listed are O(1) streaming:

| Class | Operators |
|---|---|
| O(k) — arg-bounded | `TakeLast`, `DropLast`, `Windowed` (size), `TopNBy`, `BottomNBy`, `TopN` |
| O(chunk/run) | `Chunked`, `ChunkedBy` |
| O(keys) — distinct-bounded | `Distinct`, `DistinctBy`, `DistinctWith`, `FoldBy`, `Union`, `Intersect`†, `Except`†, `Tally*`, `ToKeySet`, `GroupBy`‡, `IndexBy`, `Associate*` |
| O(all) — entire input | `Sorted*`, `SortedBy*`, `SortedWith`, `Reversed`, `Cycle`, `Unzip`, `JoinBy` (right side), `Collect`, `ToList`, `CollectAll`, `Partition` |

† plus full buffer of `b`. ‡ O(keys) map + all elements in buckets — `GroupBy` retains everything; `FoldBy` is the streaming alternative.

**Drain classes** (L7) are marked per signature in §6.3–6.6: unmarked = ∞-safe, `◐` = conditional, `⚠∞` = full drain.

---

## 7. What we cannot have

### 7.1 Constraint tightening

No method on `Seq[T any]` may require anything of `T`. Permanently method-less: `Distinct` · `Dedupe` · `Sorted` · `Sum` · `Product` · `Average` · `Max` · `Min` · `MinMax` · `TopN` · `Contains` · `IndexOf` · `ToKeySet` · `Tally` · `NonZero` · `AssociateWith` · `Equal` · `Union` · `Intersect` · `Except` · `Seq2.ToMap` · `Unzip`.

All present as package functions. Cost: one broken link mid-chain, or use the `-By` variant.

**Do not fix this with constraint tiers.** `Ord[T cmp.Ordered]` with a real `.Max()` does not compose: `Ord[T].Map(f)` must return `Seq[U]` because `U` is unconstrained, so the chain degrades after one transform and cannot get back. You would redeclare the whole method set per tier and still lose. One `Seq[T any]` plus free functions is the only design closed under chaining. Settled.

### 7.2 Receiver shape

`Flatten()` · `FlattenSlices()` · `Try.Flatten()` · `Seq2.Unzip()` — the receiver's type parameter cannot be constrained to a shape. `FlatMap(Self)` and package functions cover it.

### 7.3 The interface restriction

Interface methods cannot declare type parameters, and generic methods cannot implement interface methods (Go 1.27 release notes, verbatim). Therefore:

- No `Sequence[T]` interface implemented by `Seq`, `List`, and a hypothetical `ParSeq`.
- No pluggable backends behind one chain.
- No LINQ `IQueryable` equivalent — expression-tree providers translating a chain to SQL are permanently off the table.
- No `reflect` access to an uninstantiated generic method (which is why the §9.5 completeness test enumerates *exported symbols*, not instantiations).

The compensation is real: the same restriction makes accidental type erasure through an interface boundary impossible. For a fully-typed library that is a feature.

### 7.4 Variadic arity

`Zip3`, `Zip4`, and n-ary combinators cannot be typed. Go has no variadic type parameters, so `Zip3[A,B,C]` must return `Seq[[3]any]` or a `Triple[A,B,C]` struct. The first violates axiom 1; the second starts a tuple-type cascade that ends in `Quad`, `Quint`, and a library nobody wants.

**Resolution: `Zip` (arity 2) is the permanent answer** — not experimental, because this rejection rests on it. For three sequences, `a.Zip(b).MapTo(pack).Zip(c)` with a caller-defined struct is explicit, typed, and honest about the cost.

### 7.5 `foldRight` on `Seq`

`iter.Seq` is forward-only push. A right fold buffers the entire sequence, which is a lie in a type named `Seq`. It exists on `List` where the cost is visible in the type. Do not add it to `Seq`.

### 7.6 Kotlin features with no Go analogue

| Kotlin | Status |
|---|---|
| `inline fun` | No equivalent. Every stage is a real closure and a real call. Not closable. §8. |
| Extension functions | `fun Iterable<Int>.sum()` is a method scoped to one instantiation — same wall as §7.1. |
| `it`, trailing lambdas | Chains run ~3x the characters. Accept it; don't invent DSL tricks to hide it. |
| `?.` `?:` nullable types | `(T, bool)` throughout. `FilterMap` returns comma-ok, not a pointer. |
| Operator overloading | `Concat`. |
| Destructuring in lambdas | `Seq2` callbacks take `(K, V)` positionally. |
| `constrainOnce()` | `Once()`, but as a debugging guard, not a default. §4.1. |
| `Sequence` reusability contract | Not attempted. §4.1. |
| `partialWindows` flag | Rejected. `Windowed` emits full windows only; trailing data is `Chunked`'s job. |

### 7.7 Deliberately omitted

Each entry here is a **decision**, recorded so silence never invites re-litigation.

- **`filterIsInstance`.** Expressible via a type assertion to a type parameter — but only meaningful when `T` is an interface, and it boxes. Use a `for` loop.
- **Parallel operations** (`ParMap`, `ParFilter`). A different library with different ordering, cancellation, and failure semantics. `ToChan` + `errgroup` is the honest Go answer.
- **Reader adapters** (`Lines(io.Reader)`, `SplitBy(r, sep)`). The I/O surface (bufio options, error policy, max token size) doesn't belong in an iterator library. The package doc *shows* the ~10-line `bufio.Scanner` → `Try[string]` producer, using the §4.2 lazy-acquisition pattern, and that example is the support commitment.
- **`Cache()` / `Memoize()`.** Hidden unbounded buffering behind an innocent name (violates axiom 2); interacts unresolvably with single-pass semantics. `Collect()` and re-chain.
- **`Shuffled` / `Sample`.** Pulls in a randomness source and its seeding policy; full-buffer shuffle is `Collect` + `rand.Shuffle` in two lines.
- **`Interleave`.** Needs `iter.Pull` per input and has three reasonable exhaustion policies; no clear winner. `Zip` + `FlatMap` expresses the common case.
- **`MapWhile`.** `Map(f).TakeWhile(ok)` or `FilterMap` cover it; the fused form earns nothing (§8.3 table discipline).
- **Complex numbers in `Numeric`.** Not ordered; half the surface would be meaningless. §2.
- **Predicate helpers** (`IsZero`, `Not`, `And`). `func(n int) bool { return n > 3 }` is verbose but obvious; a `fn` package trades one kind of noise for another.
- **A 200-method surface.** Kotlin has one because extensions are free. The point of generic methods is that `s.` autocompletes usefully. What has to stay autocompletable is `Seq`'s own method set, and that lands at 84. Across the four types the library exposes 259 exported symbols, but 77 of those are the mechanically generated `List` mirror, leaving 182 distinct operations — and that is the ceiling.
- **Reimplementing `slices` / `maps`.** No `BinarySearch`, no `Compare`. `Collect()` is the handoff.

### 7.8 Shape-growing method signatures (found in implementation)

**No method on `Seq[T]` may mention `Seq[[]T]` (or any `T`-derived instantiation that grows) in its signature.** The compiler must materialize the method set of every named-type instantiation it touches; a `Chunked` method on `Seq[T]` returning `Seq[[]T]` forces `Seq[[]T]`, whose own `Chunked` forces `Seq[[][]T]`, forever — Go rejects it as an *instantiation cycle*, even for a non-generic method. `iter.Seq[[]T]` is fine (no methods); `Seq[[]T]` as a plain type in user code is fine too, once no method creates the growth edge.

Consequence: `Chunked`, `ChunkedBy`, and `Windowed` are package functions (§6.4). The chain continues normally afterward — `catena.Chunked(s, 100).Map(...)` works, because `Seq[[]T]`'s methods only ever mention non-growing instantiations. The same rule automatically protects `List` from `List[List[T]]`.

---

## 8. Performance

### 8.1 Expectations, stated up front

Range-over-func costs roughly 2–5ns per element per stage from the non-inlined indirect call. A 4-stage pipeline over 1M elements carries ~10–20ms of pure overhead before doing any work.

**The library loses to a hand-written loop on short pipelines over small in-memory data, and that is fine.** It earns its place at four-plus stages, over I/O, on unbounded input, or where intermediate types change enough that a loop needs named temporaries. Publish the benchmark showing the loss. A library that hides its cost gets used in the wrong place.

**Generic-method call cost: measured, and it's a non-issue** (updated v2.1). Generic methods benchmark identically to generic package functions, and in-package generic copies of raw operator code run at raw-concrete speed — generics carries no per-element penalty for these shapes.

**Direct-call composition (v2.2 implementation law).** Operators do not use `range` statements over their source: `for v := range src(s)` inside an operator pays the compiler's rangefunc state-machine lowering per stage. Instead every operator invokes the source directly — `src(s)(func(v T) bool { ... })` — a plain call chain. Measured effect on a 4-stage pipeline over 100k elements: 355µs → 262µs, which is 25% *faster* than the same pipeline hand-built from raw `iter.Seq` closures (350µs). Three-way baselines (hand loop / raw closures / catena) live in bench_baseline_test.go; the standing requirement is **catena ≤ raw wherever the two compile on equal terms** — any unexplained excess is a library bug, not a language cost. Post-rewrite: `Sum` 70µs vs 107µs hand loop; `Contains` 70µs vs 116µs raw; residual single-stage deltas (+4–13%) are cross-package inlining context, proven by in-package generic twins running at raw speed.

**Tooling lag**: gopls, staticcheck, and golangci-lint support for generic methods will trail the 1.27 release by months. Expect spurious diagnostics in editors; CI pins tool versions that are known-clean and documents the state in CONTRIBUTING.

### 8.2 Allocation model

- One closure per stage, allocated at chain construction. A 5-stage pipeline is 5 allocations regardless of element count.
- Zero allocations per element in streaming operators (the C8 "streaming" class).
- `List` transforms preallocate exactly: `make([]U, 0, len(l))`, no append growth.
- `Chunked` / `ChunkedBy` / `Windowed` allocate one slice per emission (L3). Non-negotiable.

### 8.3 Fusion

Fused operators exist specifically to collapse work, and each earns its place by a measurable margin:

| Fused | Replaces | Saves |
|---|---|---|
| `FilterMap` | `Map().Filter()` | one stage, one closure |
| `FoldBy` | `GroupBy()` + fold per bucket | the intermediate `map[K][]T` entirely |
| `TallyBy` | `GroupBy()` + `len` | same |
| `CountWhere` | `Filter().Count()` | one stage |
| `TopNBy` | `SortedBy().Take(n)` | O(n) memory → O(k) |
| `MinMaxOf` | `MinOf()` + `MaxOf()` | one full pass |
| `DedupeBy` | `DistinctBy()` on sorted input | unbounded memory → O(1) |
| `ChunkedBy` | `GroupBy()` on sorted input | full buffering → O(longest run) |

### 8.4 Stenciling

GC-shape stenciling means `Seq[*User]` and `Seq[*Order]` share one compiled body plus runtime dictionaries — every pointer type collapses to one shape. Full source typing, not full monomorphization. You do not get what C# value generics or Rust would give you. Relevant only if you benchmark and are surprised, but say it in the docs so nobody is.

---

## 9. Conformance suite

This is what "right the first go" means mechanically. Implementation of an operator is not done until its registration is green.

### 9.1 Harness family

Five API families need covering — v1's single `Seq→Seq` signature could not reach terminals, `Seq2`, `Try`, or `List`. Two of them carry enough shared structure to be worth a registry-driven harness, and the other three are covered by named suites:

```go
// Registry-driven. Each *_test.go registers its file's operators from an
// init(), so the file/test pairing is visible in the file list.
func conformSeq(t *testing.T, c seqOp)   // Seq→Seq: C1-C3, C5-C9, C12, C13
func conformTerm(t *testing.T, c termOp) // terminals: C1, C7, C13, and C3 via c.infOp
```

`seqOp` declares the operator's oracle, alloc class, laziness allowance and drain flag, so each case opts into the invariants its class permits — a full-drain intermediate skips C3, C8 and C12 rather than asserting something false about itself. `termOp` sets `infOp` only for conditional-drain terminals, since a full-drain terminal over an infinite source would hang by design.

`Seq2` (`TestSeq2Operators`), `Try` (the per-rule `TestTryR1`–`R5` suites, C14) and the generated `List` mirror (the `c15Cases` table against each method's `Seq` twin, C15) are covered by named tests instead: their invariants are family-specific enough that a shared registry would carry more flags than cases. Operators crossing families (`Seq→Seq2`, `Seq→Try`) register through thin adapters onto the nearest registry. The completeness test (§9.5) is what ties the five together — it maps every exported symbol to the test covering it and fails the build on any symbol nothing claims.

### 9.2 Named fixtures

Four sources, used by every harness:

- **`tracked(n)`** — yields n elements, counts `defer` executions; proves producer cleanup ran (C5).
- **`infinite()`** — never stops on its own; proves termination propagation (C3).
- **`singleUse(n)`** — panics on second iteration; proves an operator consumes its source at most once (C13).
- **`monitor(inner)`** — wraps any source, panics if `yield` is called after returning false (C6).

### 9.3 Invariants

| # | Invariant | Catches |
|---|---|---|
| C1 | Nil receiver / nil package-function argument yields empty, no panic (incl. `Pull`, `ToChan` special cases §4.6) | §4.6 violations |
| C2 | Re-iterating the result gives identical output (`Once()` registered inverted: second pass must panic) | **L1** — the captured-state bug |
| C3 | `Take(1)` on the result over `infinite()` terminates | **L2** — termination not propagated |
| C4 | Early break leaves no goroutine running (incl. `Zip`/`Equal` pull coroutines — `stop` called on every path) | L5 |
| C5 | Early break runs the producer's deferred cleanup (`tracked` counter) | §4.2 |
| C6 | `yield` never called after returning false (`monitor`) | L2 |
| C7 | Output equals the hand-written-loop oracle | correctness |
| C8 | Allocations match the operator's declared `AllocClass` (streaming = 0/element via `testing.AllocsPerRun`; bounded and unbounded classes assert their bound shape, not zero) | §8.2 regressions |
| C9 | Panic in the callback propagates uncaught | §4.4 |
| C10 | Empty input produces empty output, no panic; construction-time panics fire for invalid args (§4.10 table) | edges |
| C11 | Single-element input | edges |
| C12 | **Laziness bound**: emitting element k consumes ≤ k+c source elements, c a small per-operator constant | operators that secretly drain |
| C13 | **Single consumption**: the operator iterates its source at most once (`singleUse`) | hidden two-pass implementations (`IsEmpty`-style peeking, naive `IfEmpty`) |
| C14 | **Try invariants**: R1 (callbacks never see errored elements — instrumented callbacks), R2/R3 counting, R4 zero-on-error, R5 first-error termination; `Collect`/`CollectAll`/`Ignore`/`Errs` policy behavior | §4.9 violations |
| C15 | **Mirror consistency**: `l.Op(x)` ≡ `FromSlice(l).Op(x).Collect()` for every generated `List` operator | codegen drift |

**C2 and C3 are the ones that matter.** Every library of this shape ships with C2 bugs because the test suite consumes each sequence once. The harness consumes twice, always.

### 9.4 Property-based coverage

On top, with `rapid`:

- `s.Filter(p).Filter(q)` ≡ `s.Filter(p && q)`
- `s.Map(f).Map(g)` ≡ `s.Map(g∘f)`
- `s.Take(n).Take(m)` ≡ `s.Take(min(n,m))`
- `s.Collect()` ≡ `s.ToList()` ≡ `slices.Collect(s.Seq())`
- `catena.Distinct(s)` ≡ `catena.Dedupe(catena.Sorted(s))` as sets
- `s.SortedBy(k)` stable: equal keys retain input order (incl. `TopNBy` tie order)
- `t.Ignore().Collect()` + `t.Errs().Count()` partition `t.CollectAll()` (on re-iterable sources)
- `Union/Intersect/Except` agree with the map-based set model

### 9.5 Completeness

A test enumerates every exported symbol by parsing the package source with `go/parser` (reflect cannot enumerate uninstantiated generic methods — §7.3) and asserts each appears in the conformance registry or a named dedicated test. An operator that exists but is unregistered fails CI. This is the mechanism that keeps §9 true over time, not a code-review convention.

---

## 10. List codegen

The `List` mirror (77 methods) is **generated**, never hand-written — hand-mirroring is where drift and stale-contract bugs accumulate after v1.

- **Generator**: `internal/gen/listgen`, invoked by `//go:generate` in the package. It parses `seq.go`'s method set (`go/ast` — the generator is a build tool, not the library; axiom 1 doesn't apply to it), applies the mechanical mirror transform (`Seq[U]` → `List[U]` returns, exact preallocation, direct implementations for the O(1) overrides `Len`/`At`/`Last`/`Reversed`/`ElementAt`), and emits `list_gen.go` plus a `list_gen_test.go` containing one C15 registration per generated method.
- **Skip list**: `Seq`-only members (`Once`, `Pull`, `ToChan`, `UntilDone`, infinite constructors) are annotated `//catena:seq-only` at the source and excluded.
- **CI check**: `go generate && git diff --exit-code` — a hand-edited generated file or a stale mirror fails the build.
- The generator ships in the repo but is not part of the public API or the version contract.

---

## 11. Packaging

**Single package `catena`.** No subpackages. The method/function split already partitions the API along a meaningful line (§7.1); adding a package split on top would partition it along a second, unrelated one. Adding a subpackage later is non-breaking; removing one is not.

**Module path**: `github.com/NerdMeNot/catena`, `go 1.27` in `go.mod`. No build tags, no pre-1.27 fallback — the fallback is `go-functional`.

**Versioning**: `v1.0.0` — cut once the conformance suite was green across every operator and the bake pipelines had run — is the first release, and the API freezes there. (`Zip` is permanent — §7.4; `JoinBy` graduated from experimental after the bake pipelines.)

**Interop, explicitly documented:**

| Direction | Call |
|---|---|
| `[]T` → `Seq[T]` | `catena.FromSlice(s)` or `catena.From(slices.Values(s))` |
| `Seq[T]` → `[]T` | `s.Collect()` |
| `Seq[T]` → stdlib | `iter.Seq[T](s)` or `s.Seq()` |
| `map[K]V` → `Seq2[K,V]` | `catena.FromMap(m)` |
| `Seq2[K,V]` → `map[K]V` | `catena.CollectMap(s)` |
| `*sql.Rows` → `Try[T]` | user-written producer, §4.2 lazy-acquisition pattern |
| `io.Reader` lines → `Try[string]` | user-written producer, §7.7 doc example |
| `Try[T]` → stdlib | `iter.Seq2[T,error](t)` |

The `From` signature accepting the literal function type means `go-functional`'s `itx.Iterator`, `iter.Seq`, and `catena.Seq` all interoperate without conversion. Say so — it lowers the adoption cost from "replace your iterator library" to "add one import."

---

## 12. Resolved decisions

Recorded so they don't get relitigated in month three. v1 rows carried forward; v2 rows below the rule.

| Question | Decision | Why |
|---|---|---|
| `Seq` as struct or func type? | Func type | `range` must work directly. Costs the size hint permanently. §2.1 |
| `Seq2` or `Pair[K,V]`? | `Seq2` | Stdlib currency; `Pair` adds a per-element mapping stage at every boundary. §2.2 |
| Constraint tiers for `Ord`/`Num`? | No | Doesn't compose past one `Map`. §7.1 |
| Error model | `Seq2[T, error]`, consumer picks policy | Both skip-and-continue and abort are legitimate. §2.3 |
| `Result[T]` type? | No | Hits the same constraint wall; `error` is already an interface. |
| Context threading | Edges only (`UntilDone` on `Seq` and `Try`) | A `select` per element to serve an I/O-only concern. §4.3 |
| Re-iteration detection | No, `Once()` guard only | Detection needs state, state needs a struct, struct breaks `range`. §4.1 |
| Buffer reuse in `Chunked` | No | Corrupts data for anyone retaining the slice. L3 |
| Sort stability | Always stable, incl. `TopNBy` | Selector sorts are usually secondary sorts. L6, §4.11 |
| Parallel ops | Out of scope | Different failure and ordering semantics; separate library. §7.7 |
| Package split | Single package | The method/function split is already the meaningful partition. §11 |
| `Zip3`+ | No; `Zip` permanent | Cannot be typed without `any` or a tuple cascade. §7.4 |
| `foldRight` on `Seq` | `List` only | Buffering the whole sequence is a lie in a lazy type. §7.5 |
| | | |
| Package name | `catena` | Project identity; one word at every call site. §0 |
| Negative counts | Panic at construction, `catena:` prefix | Fail fast at the caller's line, like `strings.Repeat`. §4.10 |
| Zero counts | Valid (`Take(0)`=empty etc.) | Zero is a value, not a mistake. §4.10 |
| `Range` step=0 / sign mismatch | Panic / empty | Computed steps shouldn't panic; zero step is always a bug. §4.10 |
| `Range` overflow | Guarded termination | An iterator that can hang on `MaxInt` edges is broken. §4.10 |
| `ElementAt(-1)` | `(zero, false)` | Comma-ok family; `FindIndex`'s -1 flows in. §4.10 |
| Empty-result slices | nil, everywhere | Matches `slices.Collect`. §4.10 |
| `Product` on empty | 1 | Multiplicative identity; documented loudly. §4.10 |
| `Try` counting | Positional (R2): errors count | Preserves "consumes ≤ n"; `.Ignore().Take(n)` spells the alternative. §4.9 |
| `Try` zero-value invariant | Obligation split: producers SHOULD, consumers MUST NOT | Absolute producer claim was unenforceable. §2.3 |
| `Must()` panic value | `panic(err)` | `recover()` gets the error, not a string. §6.6 |
| `WrapErr` nil return | Keeps original error | Never convert a failure into a zero-value success. §6.6 |
| Map collision policy | Last wins (`IndexBy`, `Associate*`, `CollectMap`) | Plain map-assignment semantics; matches `maps.Collect`. §4.11 |
| `Distinct`/tie policy | First wins | Kotlin semantics; encounter order preserved. §4.11 |
| Set-op semantics | Set (deduped), encounter order | Multiset union has no obvious meaning for `Union(a, a)`. §4.11 |
| NaN | `cmp.Compare` ordering everywhere | One convention; aggregates agree with sorts. §4.12 |
| `Zip`/`Equal` mechanism | `iter.Pull`, L5 carve-out, `defer stop()` | The only way to co-iterate two push sequences. L5 |
| `Scan` initial value | Not yielded | Rust semantics; prepend if wanted. §6.2 |
| `IsEmpty` peeking | Documented destructive consume | Detection-free alternative doesn't exist. §4.1 |
| `List.Append` aliasing | Always fresh backing | One aliasing exception among fresh-allocating transforms is a trap. §6.7 |
| `List` mirror | Generated (`go:generate` + C15 + CI diff check) | Hand-mirroring is where drift lives. §10 |
| Nil callbacks | Panic, undefended | Documented; not worth a check per construction. §4.4 |
| `reflect` in tests | Permitted, `_test.go` only, for §9.5 | The completeness test is what keeps §9 true. Axiom 1 |
| `Seq2.Collect` | Dropped | Duplicated `Unzip` with a misleading name. §6.5 |
| Renames | `Step`, `NonZero`, `Get`, `IfEmpty`, `JoinToString` | §3 law + cross-ecosystem collision rule. §0 |
| Reader adapters, `Cache`, `Shuffled`, `Interleave`, `MapWhile`, complex `Numeric` | Rejected, recorded | Silence invites re-litigation. §7.7 |
| `ChunkedBy`, `Errs`, `ForEachErr`, `Try.UntilDone`, `Try/Seq2.Pull`, `Seq2.First`, `Empty2`, `EmptyTry` | Added | Each closes a real gap surfaced in review. §6 |
| Batch operators as methods? | No — package functions | `Seq[T]` → `Seq[[]T]` methods are instantiation cycles in Go 1.27. §7.8 |
| Generic-method call cost | Measured: identical to package functions | Phase 0 benchmark, Apple M5 Max: 303µs vs 303µs over 100k elements. §8.1 |
| `GenerateWhile` final value | A value produced with ok=false is not yielded | "Until next reports false"; the seed alone is unconditional. §6.1 |
| `JoinBy` graduation | Graduated before v1.0.0 | Bake evidence: ~1.5× a hand hash join (one iterator stage), clean composition in the log pipeline. §6.2 |
| Operator composition | Direct source invocation, never `range` | The rangefunc state machine costs per stage; direct calls beat the raw mechanism. §8.1 |

---

## 13. Implementation timeline

Every phase has a **machine-checkable exit criterion**: the conformance suite decides when a phase is done, not judgement. That is what makes the order safe to follow and the progress honest. ~14 working days to the first release candidate.

| Phase | Days | Scope | Exit criterion |
|---|---|---|---|
| **0 — Foundations** | 1 | `git init`, `go.mod` (go 1.27), CI (test + vet + `go generate` diff check + bench), stenciling benchmark: generic-method call vs package-function call (§8.1) | CI green on empty package; benchmark numbers recorded in `bench/README` |
| **1 — Conformance harness** | 1–2 | Five harness shapes, four fixtures (§9.2), C1–C15 implementations, §9.5 completeness test | Harness passes against 3 hand-written reference ops (`Filter`, `Take`, `Collect`); completeness test fails on an unregistered dummy |
| **2 — Seq stateless** | 3 | `Filter` family, `Map` family, `FilterMap`, `FlatMap*`, `Take/Drop(+While)`, `Concat`/`Append`/`Prepend`/`Chain`, `OnEach`, constructors | All registered; C1–C13 green |
| **3 — Seq stateful** | 4 | `Distinct*`, `Dedupe*`, `Chunked(+By)`, `Windowed`, `Scan`, `WithIndex`, `Step`, `ZipWithNext`, `Intersperse`, `TakeLast`/`DropLast`, `IfEmpty`, `Sorted*`, `Reversed`, `Cycle`, `Once` | **C2 green on every one** (the L1 phase); `Once` passes inverted C2 |
| **4 — Terminals + folds** | 5 | Search, predicate, fold (incl. `FoldBy`), grouping, aggregation (incl. `TopNBy`/`BottomNBy`) | `ConformTerminal` oracles green; §4.10 empty/edge rows green |
| **5 — Package functions** | 6 | Constraint-bound set, `Zip`/`Equal` (Pull carve-out, C4), `Union`/`Intersect`/`Except`, `NonZero` | §9.5 completeness green across the package |
| **6 — Try** | 7–8 | Full §6.6 incl. `Errs`, `UntilDone`, `Pull`; §4.2 `Rows` pattern as a testable doc example | **C14 green** — the highest-risk phase; R1–R5 verified per operator |
| **7 — Seq2 bridge** | 9 | Full §6.5 | Registered, green |
| **8 — List codegen** | 10–11 | `listgen` generator, generated mirror, list-only methods (`Get`, `Append` fresh-backing, `FoldRight`) | **C15 green for every mirrored op**; `go generate` idempotent in CI |
| **9 — Experimental + properties** | 12 | `JoinBy`; `rapid` property suite (§9.4) | Property suite green |
| **10 — Bench + docs + release** | 13–14 | Benchmarks vs hand-written loops (published in README), doc-comment audit against §6.8 tables, examples, release candidate | Every doc comment carries its contract markers; README benchmark table published |

**Post-implementation bake**: run at least one real pipeline (the `sql.Rows` or log-scan use case) against the library; fix what contact with reality surfaces; then `v1.0.0` and the API freezes. Experimental (`JoinBy`) either graduates or is cut at that gate — not later.

**Standing risk log**: (1) generic-method compiler performance is unproven — Phase 0 measures before anything depends on the answer; (2) tooling (gopls/linters) may misreport generic methods for months — pin known-clean versions in CI; (3) if a 1.27.x point release changes generic-method behavior, the conformance suite is the regression net.
