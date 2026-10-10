# Error handling

`Try[T]` is `iter.Seq2[T, error]`: a lazy sequence whose elements each
either succeeded or carry an error. The design's central choice is that
**the pipeline does not decide what an error means — the consumer does**,
because both policies are legitimate in the same codebase: a malformed
line in a log scan should probably be skipped, a failed row scan should
probably abort.

```go
vals, err  := t.Collect()     // stop at the first error; partial slice + error
vals, errs := t.CollectAll()  // drain everything, gather all errors
vals       := t.Ignore().Collect()  // drop failures, continue as a plain Seq
vals       := t.UntilErr(&err).Collect() // stop at the first error, as a plain Seq
errs       := t.Errs().Collect()    // the other half: just the errors
```

## The five rules

Every `Try` operator follows the same rule set, so nothing needs looking
up per operator. They come from one idea: **an errored element is opaque
data to an intermediate and meaningful to a terminal.** Rules 1–3 are the
first half, rule 5 is the second; rule 4 is a producer convention.

1. **Intermediates never inspect errored elements.** `Filter`'s predicate
   and `Map`'s function are simply not called on them; the errored element
   flows through untouched.
2. **Counting is positional.** `Take(3)` takes three *elements*, errored
   or not — so it consumes at most three, which is what keeps termination
   reasoning intact. The spelling for "three successes" is
   `.Ignore().Take(3)`. Note the deliberate asymmetry with `Count`: `Take`
   is an intermediate and counts errored elements, while `Count` is a
   terminal and stops at the first error (rule 5), returning the successes
   before it.
3. **An error never terminates `TakeWhile`.** Only a successful element
   failing the predicate ends the sequence.
4. **Operators that generate an error yield the zero value with it.**
   When `err != nil`, don't read the value — that's the one obligation on
   consumers.
5. **Single-error terminals stop at the first error** (`Collect`, `Fold`,
   `ForEach`, `Err`, `Count`). Only `CollectAll` and the intermediates
   continue past errors.

## `Try` is deliberately small

`Try` carries 13 intermediates, four policy exits back to `Seq` (`Ignore`,
`Errs`, `UntilErr`, `Must`) and the collecting terminals — against the 41 chainable
operators `Seq` has. That is the design, not an omission: the
moment data can fail, you should pick an error policy early and get the
full API back. The intended shape is to stay in `Try` only while errors are
still in play, then commit — `Ignore()`, `UntilErr(&err)`, `Must()`, `Collect()`,
`CollectAll()` — and continue on `Seq`.

If you find yourself wanting `Sorted` or `GroupBy` on a `Try`, the errors
have been carried one stage too far.

## Stopping at the first error without leaving the chain

`Collect` stops at the first error, but it ends the pipeline. `UntilErr`
is the same policy as a sequence: it passes elements through until the
first error, stops, and leaves that error where you told it to.

```go
var err error
byTeam := rows.UntilErr(&err).GroupBy(team) // full Seq API from here on
if err != nil {
    return err // the rows up to the failure were grouped; decide what that means
}
```

Check the error once the pipeline has been consumed — the same shape as
`bufio.Scanner`, whose `Err` is read after the loop. It is how a fallible
source feeds a stage that takes a plain `Seq`, including the concurrent
ones: `rows.UntilErr(&err).Then(parallel.Map(ctx, 8, enrich))`.

## Adding context where it exists

A scan pipeline five stages deep produces errors with no positional
information. `WrapErr` at the stage that *has* the position is the
difference between a debuggable error and a useless one:

```go
rows.WrapErr(func(err error) error {
    return fmt.Errorf("orders row %d: %w", n, err)
})
```

`Recover` turns chosen errors back into values; `OnError` is the logging
tap; `Must` converts the first error into a panic carrying the error value
itself — for pipelines where failure is a programming bug.

## Writing a producer that owns a resource

There is no `Close` on an iterator, and adding one would break `range`.
The pattern that works is **lazy acquisition**: open the resource *inside*
the iteration closure, so a pipeline that is built but never consumed
holds nothing, and `defer` fires even when a downstream stage stops early.

```go
func Rows[T any](open func() (*sql.Rows, error), scan func(*sql.Rows) (T, error)) catena.Try[T] {
    return func(yield func(T, error) bool) {
        var zero T
        rows, err := open()
        if err != nil {
            yield(zero, err)       // acquisition failure is the first element
            return
        }
        defer rows.Close()         // runs on normal exit AND early termination
        for rows.Next() {
            v, err := scan(rows)
            if err != nil {
                v = zero
            }
            if !yield(v, err) {
                return             // the defer fires
            }
        }
        if err := rows.Err(); err != nil {
            yield(zero, err)
        }
    }
}
```

That function ships as `sqlsource.Rows`, next to `source.Lines` for the
lines of a reader and `source.Paginate` for a paginated API — see the
[sources reference](operators/16-sources.md). It is written out here
because the same shape wraps anything else you own.

Two properties carry the whole design, and both are tested against a real
`database/sql` driver:

- **Never consumed ⇒ never opened.** Take an opener, not an open resource.
- **Early termination closes.** A downstream `Take(3)` returns through the
  producer, which runs its defers — composing through any number of
  stages.

The same shape works for files, network readers, and anything else that
must be released. Cancellation enters the same way, at the edge:
`s.UntilDone(ctx)` passes elements through until the context is done, then
yields the context's error and stops.

One deliberate hole to know about: `Pull` inverts control, and if you
never call its `stop`, the producer never returns and its defers never
run. `Pull` transfers ownership to you; the doc comment says so in
capitals.
