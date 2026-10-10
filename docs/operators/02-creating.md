# Creating a sequence

Where a pipeline starts. Some of these are re-iterable and some are single-use — the difference is a property of the producer, not of Seq, and each one says which below. Repeat, Generate and Cycle are infinite, so they need something downstream that stops.

## Of

```go
func Of[T any](vals ...T) Seq[T]
```

Of returns a re-iterable Seq over the given values.

```go
// For a handful of values known at the call site: the regions to
// probe, turned into the URLs to fetch.
regions := catena.Of("eu-west-1", "us-east-1")

regions.
	Map(func(r string) string { return "https://" + r + ".api.example.com/health" }).
	ForEach(func(url string) { fmt.Println(url) })
```

```
https://eu-west-1.api.example.com/health
https://us-east-1.api.example.com/health
```

## FromSlice

```go
func FromSlice[T any](s []T) Seq[T]
```

FromSlice returns a re-iterable Seq over s. The slice is not copied; mutations to it are visible to later iterations.

```go
// The slice is not copied, which is what makes this free — and makes
// the pipeline a live view: finish a job and the next iteration sees
// it.
type job struct {
	Name string
	Done bool
}
jobs := []job{{"lint", true}, {"test", false}, {"deploy", false}}
pending := catena.FromSlice(jobs).
	FilterNot(func(j job) bool { return j.Done }).
	Map(func(j job) string { return j.Name })

fmt.Println(pending.Collect())
jobs[1].Done = true
fmt.Println(pending.Collect())
```

```
[test deploy]
[deploy]
```

## From

```go
func From[T any](seq func(func(T) bool)) Seq[T]
```

From adapts any push-function sequence — iter.Seq, catena.Seq, or a third-party alias — with no conversion at the call site. Re-iterability depends on the source.

```go
// Takes the literal function type, so any iterator adapts without a
// conversion at the call site — including the standard library's.
path := "/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin"

fmt.Println(catena.From(strings.SplitSeq(path, ":")).
	Filter(func(dir string) bool { return strings.HasPrefix(dir, "/usr/") }).
	Collect())
```

```
[/usr/local/bin /usr/bin]
```

## FromMap

```go
func FromMap[K comparable, V any](m map[K]V) Seq2[K, V]
```

FromMap returns a re-iterable Seq2 over m, in undefined (map) order.

```go
// Which products are sold out? A map iterates in no particular
// order, so sort before anything order-sensitive such as printing.
stock := map[string]int{"keyboard": 12, "monitor": 0, "webcam": 0, "mouse": 40}
soldOut := catena.FromMap(stock).
	Filter(func(_ string, qty int) bool { return qty == 0 }).
	Keys()

fmt.Println(catena.Sorted(soldOut).Collect())
```

```
[monitor webcam]
```

## From2

```go
func From2[K, V any](seq func(func(K, V) bool)) Seq2[K, V]
```

From2 adapts any push-function pair sequence. Re-iterable iff the underlying source is.

```go
// Any func(yield func(K, V) bool) adapts — here the standard
// library's index/value iterator, for progress labels.
steps := []string{"checkout", "build", "test"}

catena.From2(slices.All(steps)).
	MapTo(func(i int, step string) string {
		return fmt.Sprintf("[%d/%d] %s", i+1, len(steps), step)
	}).
	ForEach(func(label string) { fmt.Println(label) })
```

```
[1/3] checkout
[2/3] build
[3/3] test
```

## FromErrs

```go
func FromErrs[T any](seq func(func(T, error) bool)) Try[T]
```

FromErrs adapts any push-function fallible sequence. Re-iterable iff the underlying source is.

```go
// Wraps a source that can fail part-way — a parser, a database
// cursor, a network stream — so the error travels with the values
// instead of being checked at every step.
fields := []string{"21.5", "22.1", "n/a", "23.0"}
readings := catena.FromErrs(func(yield func(float64, error) bool) {
	for _, f := range fields {
		if !yield(strconv.ParseFloat(f, 64)) {
			return
		}
	}
})

vals, err := readings.Collect()
fmt.Println(vals, err)
```

```
[21.5 22.1] strconv.ParseFloat: parsing "n/a": invalid syntax
```

## FromChan

```go
func FromChan[T any](ctx context.Context, ch <-chan T) Seq[T]
```

FromChan yields values received from ch until ch is closed or ctx is done. Single-use. No goroutine is started; a sequence that is never consumed never receives.

```go
results := make(chan string, 3)
for _, name := range []string{"a.png", "b.png", "c.png"} {
	results <- "resized " + name
}
close(results)

// Drains a worker pool's results until the channel closes or ctx is
// done. Single-use, and it starts no goroutine: a sequence that is
// never consumed never receives.
fmt.Println(catena.FromChan(context.Background(), results).Collect())
```

```
[resized a.png resized b.png resized c.png]
```

## Empty

```go
func Empty[T any]() Seq[T]
```

Empty returns the empty Seq. Re-iterable.

```go
// The "nothing to report" return value: callers keep chaining on it
// without a nil check.
ordersFor := func(customer string) catena.Seq[string] {
	if customer == "" {
		return catena.Empty[string]()
	}
	return catena.Of("A-1001", "A-1002")
}

fmt.Println(ordersFor("").Count(), ordersFor("ada").Count())
```

```
0 2
```

## Empty2

```go
func Empty2[K, V any]() Seq2[K, V]
```

Empty2 returns the empty Seq2. Re-iterable.

```go
// The pair-sequence counterpart: a lookup with no matches still
// collects to a usable, empty map.
labels := catena.Empty2[string, string]()

fmt.Println(labels.Count(), catena.CollectMap(labels))
```

```
0 map[]
```

## EmptyTry

```go
func EmptyTry[T any]() Try[T]
```

EmptyTry returns the empty Try. Re-iterable.

```go
// A fallible source with nothing in it, such as a stubbed query in a
// test: no rows, and no error either.
rows, err := catena.EmptyTry[string]().Collect()

fmt.Println(len(rows), err)
```

```
0 <nil>
```

## Once1

```go
func Once1[T any](v T) Seq[T]
```

Once1 returns a re-iterable Seq of exactly one value. (Once, without the suffix, is the single-use guard method on Seq.).

```go
// Lifts the one value you have into a sequence — here a header row
// to put ahead of the data. Once1, not Once: Once is the single-use
// guard method on Seq.
rows := catena.Of("ada,36", "bob,41")

fmt.Println(catena.Once1("name,age").Concat(rows).Collect())
```

```
[name,age ada,36 bob,41]
```

## Repeat

```go
func Repeat[T any](v T) Seq[T]
```

Repeat yields v forever. Re-iterable. Infinite: pair with Take or a conditional terminal.

```go
// Infinite, so it must be bounded by something downstream — here
// Zip, which stops when the hosts run out, leaving every host with
// the same starting status.
hosts := catena.Of("web-1", "web-2", "db-1")

fmt.Println(catena.CollectMap(hosts.Zip(catena.Repeat("pending"))))
```

```
map[db-1:pending web-1:pending web-2:pending]
```

## RepeatN

```go
func RepeatN[T any](v T, n int) Seq[T]
```

RepeatN yields v exactly n times. Re-iterable. Panics if n is negative.

```go
// A fixed retry schedule: three attempts, the same wait before each.
fmt.Println(catena.RepeatN(500*time.Millisecond, 3).Collect())
```

```
[500ms 500ms 500ms]
```

## Generate

```go
func Generate[T any](seed T, next func(T) T) Seq[T]
```

Generate yields seed, then next(seed), then next(next(seed)), forever. Infinite. Re-iterable iff next is pure.

```go
// Exponential backoff. The seed is yielded first, then next applied
// repeatedly — forever, so Take decides how many retries there are.
backoff := catena.Generate(100*time.Millisecond, func(d time.Duration) time.Duration {
	return d * 2
})

fmt.Println(backoff.Take(5).Collect())
```

```
[100ms 200ms 400ms 800ms 1.6s]
```

## GenerateWhile

```go
func GenerateWhile[T any](seed T, next func(T) (T, bool)) Seq[T]
```

GenerateWhile yields seed unconditionally, then successive next values until next reports false. Re-iterable iff next is pure.

```go
// Follows an API's "next page" links until there are none. The seed
// is yielded unconditionally; a value produced alongside ok=false —
// the empty link after the last page — is not.
nextLink := map[string]string{
	"/orders?page=1": "/orders?page=2",
	"/orders?page=2": "/orders?page=3",
}
pages := catena.GenerateWhile("/orders?page=1", func(url string) (string, bool) {
	next, ok := nextLink[url]
	return next, ok
})

fmt.Println(pages.Collect())
```

```
[/orders?page=1 /orders?page=2 /orders?page=3]
```

## Range

```go
func Range[I Integer](start, stop, step I) Seq[I]
```

Range yields start, start+step, ... while the value is before stop (exclusive). Re-iterable. step == 0 panics at construction; a sign mismatch between step and the start→stop direction yields an empty sequence. Termination is overflow-guarded: a step past the type's edge stops rather than wrapping. Unsigned types cannot step downward.

```go
// Half-open, like a slice expression: the offsets of each page of 90
// results, then a countdown. A sign mismatch between step and
// direction yields nothing rather than panicking, so a computed step
// is safe.
total, pageSize := 90, 25

fmt.Println(catena.Range(0, total, pageSize).Collect())
fmt.Println(catena.Range(3, 0, -1).Collect())
fmt.Println(catena.Range(0, total, -pageSize).Collect())
```

```
[0 25 50 75]
[3 2 1]
[]
```

## Cycle

```go
func Cycle[T any](s Seq[T]) Seq[T]
```

Cycle yields s over and over, forever. An empty s yields an empty Cycle — it terminates rather than spinning.

:::caution
The first pass is buffered and replayed (⚠ unbounded memory in len(s)), so Cycle is re-iterable iff that first pass of s is
:::

```go
// Round-robin: Zip stops when the tickets run out, so the endless
// rota is never a problem. Over an empty source Cycle terminates
// rather than spinning.
tickets := catena.Of("T-101", "T-102", "T-103", "T-104", "T-105")
rota := catena.Cycle(catena.Of("ada", "bob"))

fmt.Println(tickets.Zip(rota).
	MapTo(func(ticket, engineer string) string { return ticket + ":" + engineer }).
	Collect())
fmt.Println(catena.Cycle(catena.Empty[string]()).Collect())
```

```
[T-101:ada T-102:bob T-103:ada T-104:bob T-105:ada]
[]
```

## Self

```go
func Self[T any](v T) T
```

Self is the identity selector: catena.Flatten(s) is s.FlatMap(Self).

```go
// The identity selector, for the -By operators when the element is
// already the key: counting responses by status code.
statuses := catena.Of(200, 404, 200, 500, 200, 404)

fmt.Println(statuses.TallyBy(catena.Self[int]))
```

```
map[200:3 404:2 500:1]
```
