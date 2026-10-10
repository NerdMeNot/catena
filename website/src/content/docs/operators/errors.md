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
// The callback only runs on rows that parsed. The bad row passes
// through as its error, so a failure is never formatted into something
// that looks like a valid receipt line.
receipts := importedOrders().Map(func(o importedOrder) string {
	return fmt.Sprintf("%s $%.2f", o.ID, float64(o.Cents)/100)
})
lines, errs := receipts.CollectAll()
fmt.Println(lines)
fmt.Println(errs)
```

```
[A-100 $25.00 A-102 $12.00 A-103 $8.00]
[order A-101: bad amount "pending"]
```

## MapErr

```go
func (t Try[T]) MapErr[U any](f func(T) (U, error)) Try[U]
```

MapErr yields f applied to each successful element; a failed call yields (zero, err); errored elements pass through.

```go
// Parse, then validate: two stages that can each fail. Errors from
// either one flow on side by side, in input order.
ports := catena.Of("8080", "https", "70000", "9090").MapErr(strconv.Atoi).
	MapErr(func(port int) (int, error) {
		if port > 65535 {
			return 0, fmt.Errorf("port %d out of range", port)
		}
		return port, nil
	})
valid, errs := ports.CollectAll()
fmt.Println(valid)
fmt.Println(errs[0])
fmt.Println(errs[1])
```

```
[8080 9090]
strconv.Atoi: parsing "https": invalid syntax
port 70000 out of range
```

## FlatMap

```go
func (t Try[T]) FlatMap[U any](f func(T) Try[U]) Try[U]
```

FlatMap yields every element of f(v) for each successful element v, in order; an errored input element passes through un-mapped.

```go
// One fallible step fans out into more fallible steps: every upload
// that opened is parsed value by value. The upload that failed to open
// passes through as its one error, in order with the inner ones.
uploads := map[string]string{"mon.csv": "12,7", "wed.csv": "3,n/a"}
open := func(name string) (string, error) {
	if body, ok := uploads[name]; ok {
		return body, nil
	}
	return "", fmt.Errorf("open %s: no such file", name)
}
quantities := catena.Of("mon.csv", "tue.csv", "wed.csv").MapErr(open).
	FlatMap(func(body string) catena.Try[int] {
		return catena.FromSlice(strings.Split(body, ",")).MapErr(strconv.Atoi)
	})
vals, errs := quantities.CollectAll()
fmt.Println(vals, len(errs))
```

```
[12 7 3] 2
```

## Filter

```go
func (t Try[T]) Filter(pred func(T) bool) Try[T]
```

Filter yields the successful elements pred admits; errored elements pass through unexamined.

```go
// Keep the orders worth reviewing by hand. The predicate is not called
// on the row that failed, and that row is not filtered out — dropping
// it would silently discard the failure.
large := importedOrders().Filter(func(o importedOrder) bool { return o.Cents >= 1000 })
orders, errs := large.CollectAll()
fmt.Println(orders)
fmt.Println(errs)
```

```
[{A-100 2500} {A-102 1200}]
[order A-101: bad amount "pending"]
```

## FilterErr

```go
func (t Try[T]) FilterErr(pred func(T) (bool, error)) Try[T]
```

FilterErr yields the successful elements pred admits; a failed pred call yields (zero, err); errored elements pass through unexamined.

```go
// A predicate that has to look something up can fail too. A failed
// lookup takes the element's place as an error instead of being read
// as "no" and quietly dropping the order.
paid := map[string]bool{"A-100": true, "A-102": false}
settled := importedOrders().FilterErr(func(o importedOrder) (bool, error) {
	isPaid, found := paid[o.ID]
	if !found {
		return false, fmt.Errorf("order %s: no payment record", o.ID)
	}
	return isPaid, nil
})
orders, errs := settled.CollectAll()
fmt.Println(orders)
fmt.Println(errs)
```

```
[{A-100 2500}]
[order A-101: bad amount "pending" order A-103: no payment record]
```

## Take

```go
func (t Try[T]) Take(n int) Try[T]
```

Take yields at most the first n elements, errored or not (R2). Panics if n is negative.

```go
// A preview of the first two rows is two rows, bad ones included —
// Take counts elements, so it never reads past n looking for
// successes. For "the first two that parsed", Ignore first.
preview, errs := importedOrders().Take(2).CollectAll()
firstTwoGood := importedOrders().Ignore().Take(2).Collect()
fmt.Println(preview, errs)
fmt.Println(firstTwoGood)
```

```
[{A-100 2500}] [order A-101: bad amount "pending"]
[{A-100 2500} {A-102 1200}]
```

## TakeWhile

```go
func (t Try[T]) TakeWhile(pred func(T) bool) Try[T]
```

TakeWhile yields elements until pred rejects a successful element; errored elements pass through and do not terminate (R3).

```go
// Latency samples up to the first slow one. The dropped sample "--"
// passes through without ending the run; only a successful element
// failing the predicate stops it.
samples := catena.Of("12", "15", "--", "18", "950", "14").MapErr(strconv.Atoi)
fast, errs := samples.TakeWhile(func(ms int) bool { return ms < 100 }).CollectAll()
fmt.Println(fast, len(errs))
```

```
[12 15 18] 1
```

## Drop

```go
func (t Try[T]) Drop(n int) Try[T]
```

Drop skips the first n elements, errored or not (R2). Panics if n is negative.

```go
// Skipping a header row. The header fails to parse like any other bad
// row, and Drop counts it all the same: it skips elements, errored or
// not.
column := catena.Of("qty", "12", "n/a", "7").MapErr(strconv.Atoi)
quantities, errs := column.Drop(1).CollectAll()
fmt.Println(quantities, errs)
```

```
[12 7] [strconv.Atoi: parsing "n/a": invalid syntax]
```

## OnEach

```go
func (t Try[T]) OnEach(f func(T)) Try[T]
```

OnEach calls f on every successful element and passes everything through.

```go
// A tap for the rows that worked — here a running total for the import
// summary — without ending the pipeline. Failed rows pass by untouched.
importedCents := 0
_, errs := importedOrders().
	OnEach(func(o importedOrder) { importedCents += o.Cents }).
	CollectAll()
fmt.Println(importedCents, len(errs))
```

```
4500 1
```

## OnError

```go
func (t Try[T]) OnError(f func(error)) Try[T]
```

OnError calls f on every error and passes everything through — a logging hook.

```go
// The logging tap, the mirror of OnEach: record each failure as it
// goes by and still choose afterwards what failures mean. Here they
// are logged, then skipped.
orders := importedOrders().
	OnError(func(err error) { fmt.Println("skipped:", err) }).
	Ignore().
	Collect()
fmt.Println(len(orders), "orders imported")
```

```
skipped: order A-101: bad amount "pending"
3 orders imported
```

## Recover

```go
func (t Try[T]) Recover(f func(error) (T, bool)) Try[T]
```

Recover offers each error to f: reporting true replaces the element with (v, nil); reporting false passes the error through unchanged.

```go
// Fall back where a sensible default exists and let the rest through:
// an unset timeout becomes 30s, a malformed one is still an error.
// Reporting true replaces the element, false lets the error continue.
errUnset := errors.New("not set")
parse := func(s string) (time.Duration, error) {
	if s == "" {
		return 0, errUnset
	}
	return time.ParseDuration(s)
}
timeouts := catena.Of("5s", "", "fast").MapErr(parse).
	Recover(func(err error) (time.Duration, bool) {
		return 30 * time.Second, errors.Is(err, errUnset)
	})
vals, errs := timeouts.CollectAll()
fmt.Println(vals, errs)
```

```
[5s 30s] [time: invalid duration "fast"]
```

## WrapErr

```go
func (t Try[T]) WrapErr(f func(error) error) Try[T]
```

WrapErr replaces each error with f(err) — the place to add positional context. If f returns nil (a caller bug), the original error is kept: an error is never converted into a zero-value success.

```go
// The parser knows what went wrong; only this stage knows where. Add
// that here. Returning nil keeps the original error rather than
// turning a failure into a zero value.
line := 0
quantities := catena.Of("12", "n/a", "7").
	MapErr(func(field string) (int, error) {
		line++
		return strconv.Atoi(field)
	}).
	WrapErr(func(err error) error {
		return fmt.Errorf("stock.csv line %d: %w", line, err)
	})
_, err := quantities.Collect()
fmt.Println(err)
```

```
stock.csv line 2: strconv.Atoi: parsing "n/a": invalid syntax
```

## UntilDone

```go
func (t Try[T]) UntilDone(ctx context.Context) Try[T]
```

UntilDone passes elements through until ctx is done, then yields (zero, ctx.Err()) and stops.

```go
// Cancellation enters at the edge. Once ctx is done the sequence
// yields ctx's error and stops, so a shutdown surfaces like any other
// failure. Here the request is cancelled while the first order saves.
ctx, cancel := context.WithCancel(context.Background())
saved, err := importedOrders().UntilDone(ctx).
	OnEach(func(importedOrder) { cancel() }).
	Collect()
fmt.Println(len(saved), err)
```

```
1 context canceled
```

## Collect

```go
func (t Try[T]) Collect() ([]T, error)
```

Collect gathers successful elements until the first error, returning the partial slice and that error (R5); (all elements, nil) on a clean drain. Nil slice for empty.

```go
// The all-or-nothing policy: stop at the first bad value. You get the
// error plus what was gathered before it — enough to say how far it
// got, and "9090" is never parsed.
ports, err := catena.Of("8080", "https", "9090").MapErr(strconv.Atoi).Collect()
fmt.Println(ports)
fmt.Println(err)
```

```
[8080]
strconv.Atoi: parsing "https": invalid syntax
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
// The import-report policy: everything that worked and everything that
// did not, in one pass. The two slices do not correspond positionally.
orders, errs := importedOrders().CollectAll()
fmt.Printf("imported %d, rejected %d\n", len(orders), len(errs))
for _, err := range errs {
	fmt.Println("rejected:", err)
}
```

```
imported 3, rejected 1
rejected: order A-101: bad amount "pending"
```

## Ignore

```go
func (t Try[T]) Ignore() Seq[T]
```

Ignore yields the successful elements, dropping errored ones.

```go
// The log-scan policy: a malformed line is not worth stopping for.
// Drop the failures and carry on as a plain Seq, with its full API.
statuses := catena.Of("200", "503", "-", "404", "500").MapErr(strconv.Atoi)
serverErrors := statuses.Ignore().
	Filter(func(code int) bool { return code >= 500 }).
	Collect()
fmt.Println(serverErrors)
```

```
[503 500]
```

## Errs

```go
func (t Try[T]) Errs() Seq[error]
```

Errs yields the errors, dropping successful elements — the dual of Ignore. Ignore and Errs on the same single-pass Try is a double consume; use CollectAll.

```go
// Only what went wrong: a validation pass that lists every bad value
// and has no use for the good ones. It is the dual of Ignore; wanting
// both from one single-pass Try is a double consume, so use CollectAll.
flags := catena.Of("true", "yes", "false", "enabled").MapErr(strconv.ParseBool)
flags.Errs().ForEach(func(err error) { fmt.Println(err) })
```

```
strconv.ParseBool: parsing "yes": invalid syntax
strconv.ParseBool: parsing "enabled": invalid syntax
```

## UntilErr

```go
func (t Try[T]) UntilErr(err *error) Seq[T]
```

UntilErr yields the successful elements up to the first error, then stops and stores that error in *err: the policy Collect applies, as a sequence that keeps chaining. *err is set to nil when an iteration begins, so once the pipeline has been consumed it holds that run's first error, or nil — check it then, the way bufio.Scanner's Err is checked after the loop. Ignore keeps going past errors and Must panics on one; this is the exit that stops quietly and tells you why.

```go
// Stop at the first failure, but as a sequence: the chain goes on
// with the full Seq API, and the error is checked once it has run —
// the way bufio.Scanner's Err is read after the loop.
var err error
revenue := importedOrders().UntilErr(&err).
	Map(func(o importedOrder) int { return o.Cents }).
	Then(catena.Sum)
if err != nil {
	fmt.Println("import stopped:", err)
}
fmt.Println("revenue so far:", revenue)
```

```
import stopped: order A-101: bad amount "pending"
revenue so far: 2500
```

## Fold

```go
func (t Try[T]) Fold[A any](init A, f func(A, T) A) (A, error)
```

Fold reduces successful elements until the first error, returning the accumulator so far and that error (R5).

```go
// A revenue total over rows that may not parse. It stops at the first
// error and returns the accumulator so far alongside it, so a partial
// total can never be mistaken for the whole.
cents, err := importedOrders().Fold(0, func(sum int, o importedOrder) int {
	return sum + o.Cents
})
fmt.Println(cents, err)
```

```
2500 order A-101: bad amount "pending"
```

## ForEach

```go
func (t Try[T]) ForEach(f func(T) error) error
```

ForEach calls f on each successful element, stopping at and returning the first of an element error or a non-nil f return (R5).

```go
// Do something fallible with each element — here, save it. Stops at
// the first failure from either side: a bad element, or the callback
// returning an error.
err := importedOrders().ForEach(func(o importedOrder) error {
	fmt.Println("saved", o.ID)
	return nil
})
fmt.Println("stopped:", err)
```

```
saved A-100
stopped: order A-101: bad amount "pending"
```

## Err

```go
func (t Try[T]) Err() error
```

Err consumes until the first error and returns it; nil on a clean drain (R5).

```go
// A pre-flight check: is every entry in the allowlist a valid address?
// Only the first error matters, and the parsed values are not needed.
allowlist := catena.Of("10.0.0.1", "10.0.0.256", "10.0.0.3").MapErr(netip.ParseAddr)
if err := allowlist.Err(); err != nil {
	fmt.Println("allowlist rejected:", err)
}
```

```
allowlist rejected: ParseAddr("10.0.0.256"): IPv4 field has value >255
```

## Count

```go
func (t Try[T]) Count() (int, error)
```

Count counts successful elements up to the first error, which is returned alongside the count so far (R5).

```go
// Count is a terminal, so it stops at the first error: this is how
// many rows were good before the import broke, not how many are good
// in total. That would be Ignore().Count().
n, err := importedOrders().Count()
fmt.Println(n, err)
fmt.Println(importedOrders().Ignore().Count())
```

```
1 order A-101: bad amount "pending"
3
```

## Must

```go
func (t Try[T]) Must() Seq[T]
```

Must yields the successful elements and panics with the error value on the first error — recover() receives the error itself.

```go
// For input you wrote yourself — a retry schedule compiled into the
// binary — where a failure is a programming bug. The panic value is
// the error itself, so recover() can inspect it.
defer func() { fmt.Println("recovered:", recover()) }()
backoff := catena.Of("1s", "5s", "30sec").MapErr(time.ParseDuration)
fmt.Println(backoff.Must().Collect())
```

```
recovered: time: unknown unit "sec" in duration "30sec"
```

## Pull

```go
func (t Try[T]) Pull() (next func() (T, error, bool), stop func())
```

Pull converts t to a pull-based iterator. THE CALLER MUST CALL stop, even if next has returned false, or resources held by t will leak.

```go
// When the consumer has to drive — one record at a time, between
// other work. A failed element still arrives with ok true; ok is false
// only at the end. Ownership transfers to you: always call stop.
next, stop := importedOrders().Pull()
defer stop()
order, err, ok := next()
fmt.Println(order, err, ok)
_, err, ok = next()
fmt.Println(err, ok)
```

```
{A-100 2500} <nil> true
order A-101: bad amount "pending" true
```

## Seq2

```go
func (t Try[T]) Seq2() iter.Seq2[T, error]
```

Seq2 converts to the stdlib iterator type. Free.

```go
// A free conversion to the standard pair iterator, for a plain range
// loop that handles each failure its own way.
ports := catena.Of("8080", "https", "9090").MapErr(strconv.Atoi)
for port, err := range ports.Seq2() {
	if err != nil {
		fmt.Println("skipping:", err)
		continue
	}
	fmt.Println("listening on", port)
}
```

```
listening on 8080
skipping: strconv.Atoi: parsing "https": invalid syntax
listening on 9090
```
