---
title: "Errors"
description: "Try is a sequence whose elements each either succeeded or carry an error, and its whole design is that the pipeline does not decide what a failure means — the consumer does, by choosing a terminal. Five rules govern the intermediates: predicates and map functions are never called on an errored element; counts are positional; an error never ends TakeWhile; operators that generate an error yield the zero value with it; and the single-error terminals stop at the first one."
sidebar:
  order: 12
---

Try is a sequence whose elements each either succeeded or carry an error, and its whole design is that the pipeline does not decide what a failure means — the consumer does, by choosing a terminal. Five rules govern the intermediates: predicates and map functions are never called on an errored element; counts are positional; an error never ends TakeWhile; operators that generate an error yield the zero value with it; and the single-error terminals stop at the first one.

## Map

```go
func (t Try[T]) Map[U any](f func(T) U) Try[U]
```

Map yields f applied to each successful element; errored elements pass through.

```go
// The callback never sees an errored element; it passes through
// untouched, so a failure is not silently mapped into a valid value.
doubled := parseAges().Map(func(n int) int { return n * 2 })
vals, errs := doubled.CollectAll()
fmt.Println(vals, len(errs))
```

```
[72 82] 1
```

## MapErr

```go
func (t Try[T]) MapErr[U any](f func(T) (U, error)) Try[U]
```

MapErr yields f applied to each successful element; a failed call yields (zero, err); errored elements pass through.

```go
// A mapping that can itself fail. Errors from either stage flow on.
tenths := parseAges().MapErr(func(n int) (int, error) {
	if n > 40 {
		return 0, errors.New("too old")
	}
	return n * 10, nil
})
vals, errs := tenths.CollectAll()
fmt.Println(vals, len(errs))
```

```
[360] 2
```

## FlatMap

```go
func (t Try[T]) FlatMap[U any](f func(T) Try[U]) Try[U]
```

FlatMap yields every element of f(v) for each successful element v, in order; an errored input element passes through un-mapped.

```go
// An errored input passes through un-mapped; inner errors flow in
// order alongside the outer ones.
pairs := parseAges().FlatMap(func(n int) catena.Try[int] {
	return catena.Of(n, n+1).MapErr(func(v int) (int, error) { return v, nil })
})
vals, errs := pairs.CollectAll()
fmt.Println(vals, len(errs))
```

```
[36 37 41 42] 1
```

## Filter

```go
func (t Try[T]) Filter(pred func(T) bool) Try[T]
```

Filter yields the successful elements pred admits; errored elements pass through unexamined.

```go
// The predicate is not called on errored elements, and they are not
// filtered out — dropping them would silently discard failures.
old := parseAges().Filter(func(n int) bool { return n > 40 })
vals, errs := old.CollectAll()
fmt.Println(vals, len(errs))
```

```
[41] 1
```

## FilterErr

```go
func (t Try[T]) FilterErr(pred func(T) (bool, error)) Try[T]
```

FilterErr yields the successful elements pred admits; a failed pred call yields (zero, err); errored elements pass through unexamined.

```go
valid := parseAges().FilterErr(func(n int) (bool, error) {
	if n < 0 {
		return false, errors.New("negative")
	}
	return n > 40, nil
})
vals, errs := valid.CollectAll()
fmt.Println(vals, len(errs))
```

```
[41] 1
```

## Take

```go
func (t Try[T]) Take(n int) Try[T]
```

Take yields at most the first n elements, errored or not (R2). Panics if n is negative.

```go
// Counts elements, errored or not — so it consumes at most n. For
// "n successes", use Ignore().Take(n).
first, _ := parseAges().Take(2).CollectAll()
successes := parseAges().Ignore().Take(2).Collect()
fmt.Println(first, successes)
```

```
[36] [36 41]
```

## TakeWhile

```go
func (t Try[T]) TakeWhile(pred func(T) bool) Try[T]
```

TakeWhile yields elements until pred rejects a successful element; errored elements pass through and do not terminate (R3).

```go
// An errored element passes through without ending the sequence;
// only a successful element failing the predicate stops it.
kept, errs := parseAges().TakeWhile(func(n int) bool { return n < 40 }).CollectAll()
fmt.Println(kept, len(errs))
```

```
[36] 1
```

## Drop

```go
func (t Try[T]) Drop(n int) Try[T]
```

Drop skips the first n elements, errored or not (R2). Panics if n is negative.

```go
rest, _ := parseAges().Drop(1).CollectAll()
fmt.Println(rest)
```

```
[41]
```

## OnEach

```go
func (t Try[T]) OnEach(f func(T)) Try[T]
```

OnEach calls f on every successful element and passes everything through.

```go
// Runs only on successes; errors pass by untouched.
seen := 0
parseAges().OnEach(func(int) { seen++ }).CollectAll()
fmt.Println(seen)
```

```
2
```

## OnError

```go
func (t Try[T]) OnError(f func(error)) Try[T]
```

OnError calls f on every error and passes everything through — a logging hook.

```go
// The logging tap, the mirror of OnEach.
logged := 0
parseAges().OnError(func(error) { logged++ }).CollectAll()
fmt.Println(logged)
```

```
1
```

## Recover

```go
func (t Try[T]) Recover(f func(error) (T, bool)) Try[T]
```

Recover offers each error to f: reporting true replaces the element with (v, nil); reporting false passes the error through unchanged.

```go
// Repair chosen errors mid-stream: reporting true replaces the
// element, false lets the error continue.
fixed := parseAges().Recover(func(err error) (int, bool) {
	return 0, strings.Contains(err.Error(), "unknown")
})
vals, errs := fixed.CollectAll()
fmt.Println(vals, len(errs))
```

```
[36 0 41] 0
```

## WrapErr

```go
func (t Try[T]) WrapErr(f func(error) error) Try[T]
```

WrapErr replaces each error with f(err) — the place to add positional context. If f returns nil (a caller bug), the original error is kept: an error is never converted into a zero-value success.

```go
// Add the context that only this stage has. Returning nil keeps the
// original error rather than turning a failure into a zero value.
wrapped := parseAges().WrapErr(func(err error) error {
	return fmt.Errorf("parsing ages: %w", err)
})
_, err := wrapped.Collect()
fmt.Println(strings.HasPrefix(err.Error(), "parsing ages:"))
```

```
true
```

## UntilDone

```go
func (t Try[T]) UntilDone(ctx context.Context) Try[T]
```

UntilDone passes elements through until ctx is done, then yields (zero, ctx.Err()) and stops.

```go
ctx, cancel := context.WithCancel(context.Background())
cancel()
_, err := parseAges().UntilDone(ctx).Collect()
fmt.Println(err)
```

```
context canceled
```

## Collect

```go
func (t Try[T]) Collect() ([]T, error)
```

Collect gathers successful elements until the first error, returning the partial slice and that error (R5); (all elements, nil) on a clean drain. Nil slice for empty.

```go
// Abort: the values gathered before the failure, plus the error.
vals, err := parseAges().Collect()
fmt.Println(vals, err != nil)
```

```
[36] true
```

## CollectAll

```go
func (t Try[T]) CollectAll() ([]T, []error)
```

CollectAll drains everything, gathering all successes and all errors. Positional correspondence between the two slices is lost. Nil slices when empty.

:::caution
Full drain
:::

```go
// Gather: everything that worked and everything that did not, in one
// pass. The two slices do not correspond positionally.
vals, errs := parseAges().CollectAll()
fmt.Println(vals, len(errs))
```

```
[36 41] 1
```

## Ignore

```go
func (t Try[T]) Ignore() Seq[T]
```

Ignore yields the successful elements, dropping errored ones.

```go
// Skip: drop the failures and carry on as a plain Seq.
fmt.Println(parseAges().Ignore().Collect())
```

```
[36 41]
```

## Errs

```go
func (t Try[T]) Errs() Seq[error]
```

Errs yields the errors, dropping successful elements — the dual of Ignore. Ignore and Errs on the same single-pass Try is a double consume; use CollectAll.

```go
// The dual of Ignore. Consuming both on one single-pass Try is a
// double consume — use CollectAll instead.
fmt.Println(parseAges().Errs().Count())
```

```
1
```

## Fold

```go
func (t Try[T]) Fold[A any](init A, f func(A, T) A) (A, error)
```

Fold reduces successful elements until the first error, returning the accumulator so far and that error (R5).

```go
// Stops at the first error, returning the accumulator so far.
sum, err := parseAges().Fold(0, func(acc, n int) int { return acc + n })
fmt.Println(sum, err != nil)
```

```
36 true
```

## ForEach

```go
func (t Try[T]) ForEach(f func(T) error) error
```

ForEach calls f on each successful element, stopping at and returning the first of an element error or a non-nil f return (R5).

```go
// Stops at the first of an element error or a callback error.
err := parseAges().ForEach(func(n int) error {
	fmt.Println("handled", n)
	return nil
})
fmt.Println(err != nil)
```

```
handled 36
true
```

## Err

```go
func (t Try[T]) Err() error
```

Err consumes until the first error and returns it; nil on a clean drain (R5).

```go
// Just the first error, if any — for pipelines run entirely for
// their side effects.
fmt.Println(parseAges().Err() != nil)
```

```
true
```

## Count

```go
func (t Try[T]) Count() (int, error)
```

Count counts successful elements up to the first error, which is returned alongside the count so far (R5).

```go
// Successes counted up to the first error, which is returned with it.
n, err := parseAges().Count()
fmt.Println(n, err != nil)
```

```
1 true
```

## Must

```go
func (t Try[T]) Must() Seq[T]
```

Must yields the successful elements and panics with the error value on the first error — recover() receives the error itself.

```go
// For pipelines where a failure is a programming bug. The panic
// value is the error itself, so recover() can inspect it.
defer func() { fmt.Println("recovered:", recover()) }()
parseAges().Must().Drain()
```

```
recovered: strconv.Atoi: parsing "unknown": invalid syntax
```

## Pull

```go
func (t Try[T]) Pull() (next func() (T, error, bool), stop func())
```

Pull converts t to a pull-based iterator. THE CALLER MUST CALL stop, even if next has returned false, or resources held by t will leak.

```go
next, stop := parseAges().Pull()
defer stop()
v, err, ok := next()
fmt.Println(v, err, ok)
```

```
36 <nil> true
```

## Seq2

```go
func (t Try[T]) Seq2() iter.Seq2[T, error]
```

Seq2 converts to the stdlib iterator type. Free.

```go
// A free conversion to the standard pair iterator.
for v, err := range parseAges().Seq2() {
	if err != nil {
		fmt.Println("error at", v)
		break
	}
	fmt.Println("ok", v)
}
```

```
ok 36
error at 0
```
