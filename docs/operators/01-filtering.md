# Filtering

Keeping some elements and dropping others — by predicate, by key, or by what has already been seen. Everything here streams: no operator on this page buffers the input, though the distinct family retains one entry per distinct key.

## Filter

```go
func (s Seq[T]) Filter(pred func(T) bool) Seq[T]
```

Filter yields the elements for which pred returns true.

```go
// The unpaid orders worth a reminder email. Nothing is tested until
// Collect asks for the result.
type order struct {
	ID    string
	Total float64
	Paid  bool
}
orders := catena.Of(
	order{"A-1001", 42.50, true},
	order{"A-1002", 310.00, false},
	order{"A-1003", 18.99, false},
	order{"A-1004", 129.00, false},
)

fmt.Println(orders.
	Filter(func(o order) bool { return !o.Paid && o.Total >= 100 }).
	Map(func(o order) string { return o.ID }).
	Collect())
```

```
[A-1002 A-1004]
```

## FilterNot

```go
func (s Seq[T]) FilterNot(pred func(T) bool) Seq[T]
```

FilterNot yields the elements for which pred returns false.

```go
// The negated form, for when the predicate reads better positively:
// "is a probe" is the natural question, and the traffic report wants
// everything that is not one.
isProbe := func(path string) bool {
	return path == "/healthz" || path == "/metrics"
}
paths := catena.Of("/checkout", "/healthz", "/cart", "/metrics", "/login")

fmt.Println(paths.FilterNot(isProbe).Collect())
```

```
[/checkout /cart /login]
```

## FilterIndexed

```go
func (s Seq[T]) FilterIndexed(pred func(int, T) bool) Seq[T]
```

FilterIndexed yields the elements for which pred(index, element) returns true. The index counts source elements from 0.

```go
// The index is the line's position in the source, not among the
// survivors: line 0 is the header however many blank lines the
// predicate has dropped along the way.
export := catena.Of("host,port", "", "db-1,5432", "", "cache-1,6379")

fmt.Println(export.
	FilterIndexed(func(i int, line string) bool { return i > 0 && line != "" }).
	Collect())
```

```
[db-1,5432 cache-1,6379]
```

## FilterErr

```go
func (s Seq[T]) FilterErr(pred func(T) (bool, error)) Try[T]
```

FilterErr yields elements pred admits, as a Try; a failed pred call yields (zero, err).

```go
// A predicate that can fail produces a Try, so the caller chooses
// what a failure means rather than the pipeline deciding. Here a bad
// config entry is reported alongside the ports that did parse.
configured := catena.Of("8080", "443", "http", "22")
privileged := configured.FilterErr(func(s string) (bool, error) {
	port, err := strconv.Atoi(s)
	return port < 1024, err
})

kept, errs := privileged.CollectAll()
fmt.Println(kept, errs)
```

```
[443 22] [strconv.Atoi: parsing "http": invalid syntax]
```

## Distinct

```go
func Distinct[T comparable](s Seq[T]) Seq[T]
```

Distinct yields elements not seen before; the first occurrence wins. Distinct is a package function, not a method, and so are the others in this file: they constrain the element type (comparable, cmp.Ordered, Numeric), and a method on Seq[T any] may require nothing of T — or, for the Flatten family, they constrain the receiver's shape. Every comparable-constrained function panics at runtime if T is an interface type holding a non-comparable value. The chain continues normally after them: catena.Distinct(s).Filter(f). On input already sorted by the compared value, Dedupe is the O(1)-memory equivalent.

:::caution
Retains one entry per distinct value — unbounded
:::

```go
// The unique client addresses in an access log, in the order they
// first appeared. Distinct constrains the element type, so it is a
// package function rather than a method — a method on Seq[T any] may
// require nothing of T.
clients := catena.Of("10.0.0.7", "10.0.0.9", "10.0.0.7", "10.0.0.4", "10.0.0.9")

fmt.Println(catena.Distinct(clients).Collect())
```

```
[10.0.0.7 10.0.0.9 10.0.0.4]
```

## DistinctBy

```go
func (s Seq[T]) DistinctBy[K comparable](sel func(T) K) Seq[T]
```

DistinctBy yields elements whose key has not been seen before; the first occurrence wins. catena.Distinct(s) is the no-selector form, for a comparable T. On key-sorted input DedupeBy is the O(1)-memory equivalent.

:::caution
Retains one key per distinct value — unbounded
:::

```go
// Page the on-call engineer once per host, not once per alert. The
// first alert seen for a host is the one that survives.
type alert struct {
	Host, Problem string
}
alerts := catena.Of(
	alert{"db-1", "disk 91% full"},
	alert{"web-3", "5xx rate high"},
	alert{"db-1", "replication lag"},
)

fmt.Println(alerts.
	DistinctBy(func(a alert) string { return a.Host }).
	Collect())
```

```
[{db-1 disk 91% full} {web-3 5xx rate high}]
```

## DistinctWith

```go
func (s Seq[T]) DistinctWith(eq func(a, b T) bool) Seq[T]
```

DistinctWith yields elements no earlier element equals under eq. First occurrence wins.

:::caution
Retains all distinct elements and compares in O(n²) — small inputs only
:::

```go
// For an equality of your own — here, signups that differ only in
// letter case are the same person — or for keys that are not
// comparable. Retains every distinct element and compares against
// all of them, so this is for small inputs.
signups := catena.Of(
	"Ada@example.com", "bob@example.com", "ada@example.com", "ADA@EXAMPLE.COM",
)

fmt.Println(signups.DistinctWith(strings.EqualFold).Collect())
```

```
[Ada@example.com bob@example.com]
```

## Duplicates

```go
func Duplicates[T comparable](s Seq[T]) Seq[T]
```

Duplicates yields the elements that occur more than once: each such value once, at its second occurrence, so in the order values were first repeated.

:::caution
Retains one entry per distinct value — unbounded
:::

```go
// Each value that occurs more than once, reported once however often
// it repeats — in the order the repeats were found.
requestIDs := catena.Of("r-71", "r-30", "r-92", "r-30", "r-71", "r-71")
fmt.Println(catena.Duplicates(requestIDs).Collect())
```

```
[r-30 r-71]
```

## DuplicatesBy

```go
func (s Seq[T]) DuplicatesBy[K comparable](sel func(T) K) Seq[T]
```

DuplicatesBy yields the elements that repeat a key: for each key seen more than once, the second element to have it, once. Order is the order in which keys were first repeated. catena.Duplicates(s) is the no-selector form, for a comparable T.

:::caution
Retains one key per distinct value — unbounded
:::

```go
// The element reported is the one that repeats the key: here, the
// second signup with each address, which is the one to reject.
type signup struct {
	Email string
	Name  string
}
signups := catena.Of(
	signup{"ada@example.com", "Ada"},
	signup{"bob@example.com", "Bob"},
	signup{"ada@example.com", "Ada L."},
)
fmt.Println(signups.DuplicatesBy(func(s signup) string { return s.Email }).Collect())
```

```
[{ada@example.com Ada L.}]
```

## Dedupe

```go
func Dedupe[T comparable](s Seq[T]) Seq[T]
```

Dedupe yields elements that differ from their predecessor — consecutive duplicates only, O(1) memory.

```go
// Like uniq(1): consecutive duplicates only, so a repeat that comes
// back later is kept. On sorted input it equals Distinct at a
// fraction of the cost; on unsorted input the two differ.
log := catena.Of(
	"connection reset", "connection reset", "connection reset",
	"reconnected",
	"connection reset",
)

fmt.Println(catena.Dedupe(log).JoinToString(" | ", catena.Self[string]))
```

```
connection reset | reconnected | connection reset
```

## DedupeBy

```go
func (s Seq[T]) DedupeBy[K comparable](sel func(T) K) Seq[T]
```

DedupeBy yields elements whose key differs from the previous element's key — consecutive duplicates only, O(1) memory. On key-sorted input it equals DistinctBy at a fraction of the cost.

```go
// Turns a stream of health checks into a stream of state changes.
// Only CONSECUTIVE repeats collapse, in O(1) memory, so the second
// "up" is kept — and it is safe on an unbounded source, where
// DistinctBy's seen-set would grow forever.
type check struct {
	At, Status string
}
checks := catena.Of(
	check{"09:00", "up"}, check{"09:01", "up"},
	check{"09:02", "down"}, check{"09:03", "down"},
	check{"09:04", "up"},
)

fmt.Println(checks.
	DedupeBy(func(c check) string { return c.Status }).
	Collect())
```

```
[{09:00 up} {09:02 down} {09:04 up}]
```

## NonZero

```go
func NonZero[T comparable](s Seq[T]) Seq[T]
```

NonZero yields the elements that are not the zero value of T.

```go
// Drops the zero value of T — the empty strings that splitting a
// path leaves behind here, but equally 0, nil pointers, or a zero
// struct.
segments := catena.FromSlice(strings.Split("/var/log//nginx/", "/"))

fmt.Println(catena.NonZero(segments).Collect())
```

```
[var log nginx]
```
