# Transforming

Changing what the elements are. These are the operators generic methods made possible: the element type changes mid-chain, and the chain keeps its methods. All of them stream — nothing here buffers.

## Map

```go
func (s Seq[T]) Map[U any](f func(T) U) Seq[U]
```

Map yields f applied to each element.

```go
// The element type changes mid-chain — accounts in, addresses out —
// which is what generic methods made possible.
type account struct {
	Name  string
	Email string
}
accounts := catena.Of(
	account{"Ada", "ada@example.com"},
	account{"Bob", "bob@example.com"},
)
fmt.Println(accounts.Map(func(a account) string { return a.Email }).Collect())
```

```
[ada@example.com bob@example.com]
```

## MapIndexed

```go
func (s Seq[T]) MapIndexed[U any](f func(int, T) U) Seq[U]
```

MapIndexed yields f(index, element), counting from 0.

```go
// The position arrives with the element, so numbering needs no counter
// kept outside the chain. It counts from 0; a ranking wants i+1.
finishers := catena.Of("kiptum", "bekele", "farah")
fmt.Println(finishers.
	MapIndexed(func(i int, name string) string { return fmt.Sprintf("%d. %s", i+1, name) }).
	Collect())
```

```
[1. kiptum 2. bekele 3. farah]
```

## FilterMap

```go
func (s Seq[T]) FilterMap[U any](f func(T) (U, bool)) Seq[U]
```

FilterMap yields the mapped value for each element f reports true for — a fused Map + Filter.

```go
// Fused filter and map: keep the error lines and strip their prefix in
// one stage. Anything returning (value, bool) fits, and the test is
// not run a second time to get the value.
logs := catena.Of(
	"INFO listening on :8080",
	"ERROR db: connection refused",
	"INFO GET /health 200",
	"ERROR cache: timeout",
)
for msg := range logs.FilterMap(func(line string) (string, bool) {
	return strings.CutPrefix(line, "ERROR ")
}).Seq() {
	fmt.Println(msg)
}
```

```
db: connection refused
cache: timeout
```

## FlatMap

```go
func (s Seq[T]) FlatMap[U any](f func(T) Seq[U]) Seq[U]
```

FlatMap yields all elements of f(v) for each element v, in order.

```go
// Each element expands into a sequence of its own and the results run
// together in order. The inner sequences stay lazy: no port list is
// built for a rule until the consumer reaches it.
type rule struct {
	Service  string
	From, To int
}
rules := catena.Of(rule{"web", 8080, 8082}, rule{"metrics", 9100, 9101})
fmt.Println(rules.
	FlatMap(func(r rule) catena.Seq[int] { return catena.Range(r.From, r.To+1, 1) }).
	Collect())
```

```
[8080 8081 8082 9100 9101]
```

## FlatMapSlice

```go
func (s Seq[T]) FlatMapSlice[U any](f func(T) []U) Seq[U]
```

FlatMapSlice yields all elements of the slice f(v) for each element v.

```go
// The same, when each element already holds a slice: every file touched
// by a run of commits, with no wrapping of the slice in a sequence.
type commit struct {
	Hash  string
	Files []string
}
commits := catena.Of(
	commit{"a1f9", []string{"go.mod", "main.go"}},
	commit{"c03d", []string{"README.md"}},
)
fmt.Println(commits.FlatMapSlice(func(c commit) []string { return c.Files }).Collect())
```

```
[go.mod main.go README.md]
```

## Flatten

```go
func Flatten[T any](s Seq[Seq[T]]) Seq[T]
```

Flatten yields every element of every inner sequence, in order.

```go
// One stream from a sequence of sequences — here, the hosts of each
// region — in order, and still lazy.
byRegion := catena.Of(
	catena.Of("eu-1.example.com", "eu-2.example.com"),
	catena.Of("us-1.example.com"),
)
fmt.Println(catena.Flatten(byRegion).Collect())
```

```
[eu-1.example.com eu-2.example.com us-1.example.com]
```

## FlattenSlices

```go
func FlattenSlices[T any](s Seq[[]T]) Seq[T]
```

FlattenSlices yields every element of every slice, in order.

```go
// The same for slices: a paginated API hands back a page at a time,
// and the caller wants the rows.
pages := catena.Of(
	[]string{"invoice-101", "invoice-102"},
	[]string{"invoice-103"},
)
fmt.Println(catena.FlattenSlices(pages).Collect())
```

```
[invoice-101 invoice-102 invoice-103]
```

## Scan

```go
func (s Seq[T]) Scan[A any](init A, f func(A, T) A) Seq[A]
```

Scan yields the running accumulator: f(init, e0), f(that, e1), ... The initial value itself is not yielded.

```go
// A running fold: the balance after every transaction rather than only
// the final one. The opening balance itself is not emitted, so the
// output is as long as the input.
opening := 500
transactions := catena.Of(120, -75, -300, 40)
fmt.Println(transactions.
	Scan(opening, func(balance, amount int) int { return balance + amount }).
	Collect())
```

```
[620 545 245 285]
```

## MapErr

```go
func (s Seq[T]) MapErr[U any](f func(T) (U, error)) Try[U]
```

MapErr yields f applied to each element as a Try; a failed call yields (zero, err).

```go
// A mapping that can fail produces a Try, so one bad timeout in a
// config does not have to abort the rest. A failed call yields the zero
// value alongside its error, never a half-built one.
timeouts := catena.Of("30s", "soon", "5m")
parsed, errs := timeouts.MapErr(time.ParseDuration).CollectAll()
fmt.Println(parsed, len(errs))
```

```
[30s 5m0s] 1
```

## OnEach

```go
func (s Seq[T]) OnEach(f func(T)) Seq[T]
```

OnEach calls f on every element and passes it through unchanged.

```go
// Side effects without changing the stream — logging, metrics,
// progress. Elements pass through untouched, and nothing is logged
// until the terminal operation pulls them.
type upload struct {
	Name  string
	Bytes int
}
uploads := catena.Of(upload{"report.pdf", 48_200}, upload{"logo.png", 9_100})
total := uploads.
	OnEach(func(u upload) { fmt.Println("uploading", u.Name) }).
	SumOf(func(u upload) int { return u.Bytes })
fmt.Println(total, "bytes")
```

```
uploading report.pdf
uploading logo.png
57300 bytes
```
