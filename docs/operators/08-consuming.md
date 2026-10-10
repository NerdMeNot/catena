# Consuming

The operators that make a pipeline run. Everything before one of these is a description of work; one of these performs it, once. Collect and ForEach drain the sequence, so neither returns on an infinite one — Pull and ToChan hand control back to you instead, and both make you responsible for stopping.

## Collect

```go
func (s Seq[T]) Collect() []T
```

Collect drains the sequence into a slice; nil for empty. ToList is the same drain returning a List, which carries the eager operator set.

:::caution
Full drain
:::

```go
// The hand-off back to ordinary Go: a plain slice. Nothing matching
// gives nil, as slices.Collect does, so len and range need no guard.
addrs := catena.Of("web-1:443", "db-1:5432", "cache-1:6379")
onPort := func(port string) func(string) bool {
	return func(addr string) bool { return strings.HasSuffix(addr, port) }
}
tls := addrs.Filter(onPort(":443")).Collect()
plain := addrs.Filter(onPort(":80")).Collect()
fmt.Println(tls, plain == nil)
```

```
[web-1:443] true
```

## ToList

```go
func (s Seq[T]) ToList() List[T]
```

ToList drains the sequence into a List; nil for empty.

:::caution
Full drain
:::

```go
// The eager twin: a List has the same operations, evaluated at once,
// so it can be measured, indexed and reused without re-running the
// pipeline.
releases := catena.Of("v1.4.0", "v1.4.1", "v1.5.0").ToList()
fmt.Println(releases.Len(), releases.At(releases.Len()-1))
```

```
3 v1.5.0
```

## Seq

```go
func (s Seq[T]) Seq() iter.Seq[T]
```

Seq converts to the stdlib iterator type. Free.

```go
// A free conversion to the standard iterator type, in both
// directions — Seq IS iter.Seq underneath — so anything in the
// standard library that takes an iterator takes a pipeline.
hosts := catena.Of("web-2", "db-1", "web-1")
fmt.Println(slices.Sorted(hosts.Seq()))
```

```
[db-1 web-1 web-2]
```

## Pull

```go
func (s Seq[T]) Pull() (next func() (T, bool), stop func())
```

Pull converts s to a pull-based iterator. THE CALLER MUST CALL stop, even if next has returned false, or resources held by s will leak.

```go
// Inverts control for hand-written loops, such as reading a header
// row before the records. THE CALLER MUST CALL stop, or a producer
// holding a resource never releases it.
rows := catena.Of("id,email", "1,ada@example.com", "2,bob@example.com")
next, stop := rows.Pull()
defer stop()

header, _ := next()
record, ok := next()
fmt.Println(header, "|", record, ok)
```

```
id,email | 1,ada@example.com true
```

## ToChan

```go
func (s Seq[T]) ToChan(ctx context.Context) <-chan T
```

ToChan starts a goroutine that sends every element on the returned unbuffered channel. The channel is closed when the sequence ends or ctx is done — the consumer must drain or cancel, or the goroutine leaks.

```go
// The fan-out mechanism: a Seq is not safe to consume from two
// goroutines, a channel is. Cancelling ctx closes the channel.
uploads := catena.Of("a.png", "b.png", "c.png", "d.png")
jobs := uploads.ToChan(context.Background())

var resized atomic.Int32
var workers sync.WaitGroup
for range 2 {
	workers.Go(func() {
		for range jobs {
			resized.Add(1)
		}
	})
}
workers.Wait()
fmt.Println(resized.Load())
```

```
4
```

## ForEach

```go
func (s Seq[T]) ForEach(f func(T))
```

ForEach calls f on every element.

:::caution
Full drain
:::

```go
// The end of a pipeline whose point is the side effect.
results := catena.Of("ok pkg/api", "FAIL pkg/db", "ok pkg/web", "FAIL pkg/auth")
results.
	Filter(func(line string) bool { return strings.HasPrefix(line, "FAIL") }).
	ForEach(func(line string) { fmt.Println(line) })
```

```
FAIL pkg/db
FAIL pkg/auth
```

## ForEachIndexed

```go
func (s Seq[T]) ForEachIndexed(f func(int, T))
```

ForEachIndexed calls f(index, element) on every element.

:::caution
Full drain
:::

```go
// The position comes with the element, counted from zero.
steps := catena.Of("back up the database", "run migrations", "restart the API")
steps.ForEachIndexed(func(i int, step string) {
	fmt.Printf("%d. %s\n", i+1, step)
})
```

```
1. back up the database
2. run migrations
3. restart the API
```

## ForEachErr

```go
func (s Seq[T]) ForEachErr(f func(T) error) error
```

ForEachErr calls f on every element, stopping at and returning the first non-nil error; nil if the sequence drains clean.

```go
// Stops at the first error the callback returns, and returns it:
// the order after the failure is never attempted.
type order struct{ ID, Card string }
orders := catena.Of(order{"A-1", "visa"}, order{"A-2", ""}, order{"A-3", "amex"})

err := orders.ForEachErr(func(o order) error {
	if o.Card == "" {
		return fmt.Errorf("order %s: no card on file", o.ID)
	}
	fmt.Println("charged", o.ID)
	return nil
})
fmt.Println("err:", err)
```

```
charged A-1
err: order A-2: no card on file
```

## Drain

```go
func (s Seq[T]) Drain()
```

Drain consumes the sequence for its side effects.

:::caution
Full drain
:::

```go
// Runs a pipeline whose stages do the work, discarding the elements:
// here, warming a cache.
type user struct {
	ID   int
	Name string
}
cache := map[int]string{}
catena.Of(user{7, "ada"}, user{9, "bob"}).
	OnEach(func(u user) { cache[u.ID] = u.Name }).
	Drain()
fmt.Println(len(cache), cache[9])
```

```
2 bob
```

## Once

```go
func (s Seq[T]) Once() Seq[T]
```

Once returns a sequence that panics if iterated more than once — a development guard for the single-pass contract, not a synchronization mechanism. This is the one operator whose state deliberately lives outside the iteration closure. (catena.Once1 is unrelated: it constructs a one-element sequence.).

```go
// A development guard for sources that can only be read once — a
// request body, a database cursor: the second consumption panics
// instead of silently re-running the producer or reading nothing.
body := catena.Of("id,total", "1,40", "2,15").Once()
fmt.Println(body.Count(), "lines")

defer func() { fmt.Println("recovered:", recover()) }()
body.Collect()
```

```
3 lines
recovered: catena: Once: sequence consumed more than once
```

## UntilDone

```go
func (s Seq[T]) UntilDone(ctx context.Context) Try[T]
```

UntilDone passes elements through until ctx is done, then yields (zero, ctx.Err()) and stops.

```go
// Cancellation enters at the edge rather than threading a context
// through every stage: an endless polling loop ends when the context
// does, and the context's error arrives as the pipeline's error.
ctx, cancel := context.WithCancel(context.Background())
polls := catena.Generate(1, func(attempt int) int { return attempt + 1 })

done, err := polls.UntilDone(ctx).
	OnEach(func(attempt int) {
		if attempt == 3 { // the deadline passes, or the user hits Ctrl-C
			cancel()
		}
	}).
	Collect()
fmt.Println(done, err)
```

```
[1 2 3] context canceled
```
