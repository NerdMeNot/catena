# Folding and grouping

Reducing a sequence to one value, or to one value per key. FoldBy is the operator that justifies the library: it aggregates per key as elements stream past, so its memory is bounded by the number of distinct keys rather than by the number of elements — where GroupBy retains everything.

## Fold

```go
func (s Seq[T]) Fold[A any](init A, f func(A, T) A) A
```

Fold reduces the sequence into an accumulator, left to right. Reduce is the same operation using the first element as the initial accumulator; FoldBy folds per key in one streaming pass.

:::caution
Full drain
:::

```go
// The seed is where the result starts: an opening balance of 100,
// then the day's deposits and withdrawals.
ledger := catena.Of(250, -75, -20)
balance := ledger.Fold(100, func(balance, amount int) int { return balance + amount })
fmt.Println(balance)
```

```
255
```

## FoldIndexed

```go
func (s Seq[T]) FoldIndexed[A any](init A, f func(int, A, T) A) A
```

FoldIndexed is Fold with the element index.

:::caution
Full drain
:::

```go
// The fold knows where it is: gather the line number of every
// failing package.
results := catena.Of("ok   pkg/api", "FAIL pkg/db", "ok   pkg/web", "FAIL pkg/auth")
failedAt := results.FoldIndexed([]int(nil), func(i int, lines []int, line string) []int {
	if strings.HasPrefix(line, "FAIL") {
		return append(lines, i+1)
	}
	return lines
})
fmt.Println(failedAt)
```

```
[2 4]
```

## FoldWhile

```go
func (s Seq[T]) FoldWhile[A any](init A, f func(A, T) (A, bool)) A
```

FoldWhile folds until f reports false; the accumulator from the stopping call is included in the result.

```go
// Stops when the callback says so; the accumulator from the stopping
// call is included. The file that tips the batch past 10 MB is
// counted, and the one after it is never looked at.
fileMB := catena.Of(4, 3, 5, 2)
batchMB := fileMB.FoldWhile(0, func(total, size int) (int, bool) {
	total += size
	return total, total < 10
})
fmt.Println(batchMB)
```

```
12
```

## FoldErr

```go
func (s Seq[T]) FoldErr[A any](init A, f func(A, T) (A, error)) (A, error)
```

FoldErr folds until f fails, returning the accumulator so far and the first error.

```go
// Stops at the first error and returns the accumulator so far: the
// total of the rows that did parse, and why the next one did not.
amounts := catena.Of("12", "30", "n/a", "7")
total, err := amounts.FoldErr(0, func(sum int, field string) (int, error) {
	n, err := strconv.Atoi(field)
	if err != nil {
		return 0, err
	}
	return sum + n, nil
})
fmt.Println(total, err)
```

```
42 strconv.Atoi: parsing "n/a": invalid syntax
```

## FoldBy

```go
func (s Seq[T]) FoldBy[K comparable, A any](
	key func(T) K, init func(K) A, f func(A, T) A,
) map[K]A
```

FoldBy folds each element into a per-key accumulator: streaming grouped aggregation with no intermediate per-key slices. init is called once per distinct key. Memory is bounded by the number of distinct keys, not elements.

:::caution
Full drain; map iteration order is undefined
:::

```go
// Streaming aggregation per key. GroupBy would retain every sale;
// this retains one running total per region.
type sale struct {
	Region string
	Amount int
}
sales := catena.Of(
	sale{"west", 100}, sale{"east", 40}, sale{"west", 20},
)
totals := sales.FoldBy(
	func(s sale) string { return s.Region },
	func(string) int { return 0 },
	func(sum int, s sale) int { return sum + s.Amount },
)
fmt.Println(totals["west"], totals["east"])
```

```
120 40
```

## Reduce

```go
func (s Seq[T]) Reduce(f func(T, T) T) (T, bool)
```

Reduce folds the sequence using its first element as the initial accumulator; (zero, false) on empty input.

:::caution
Full drain
:::

```go
// Uses the first element as the seed, so it suits values with no
// natural zero — and reports false on empty rather than inventing one.
type window struct{ From, To int }
merge := func(a, b window) window {
	return window{min(a.From, b.From), max(a.To, b.To)}
}
outages := catena.Of(window{13, 14}, window{9, 11}, window{16, 17})

span, ok := outages.Reduce(merge)
_, nonEmpty := catena.Empty[window]().Reduce(merge)
fmt.Println(span, ok, nonEmpty)
```

```
{9 17} true false
```

## FoldedBy

```go
func (s Seq[T]) FoldedBy[K comparable, A any](
	key func(T) K, init func(K) A, f func(A, T) A,
) Seq2[K, A]
```

FoldedBy is FoldBy that stays in the chain: it folds each element into a per-key accumulator, then yields each key with its accumulator in the order the keys were first seen. init is called once per distinct key. Memory is bounded by the number of distinct keys, not elements. catena.GroupedBy is the same shape with every element kept.

:::caution
Fully drains the source before the first emission — hangs on infinite input
:::

```go
// FoldBy that stays in the chain: keys come out in the order they
// were first seen, so the result is deterministic and keeps flowing.
type hit struct {
	Path  string
	Bytes int
}
hits := catena.Of(hit{"/checkout", 900}, hit{"/home", 2400}, hit{"/health", 2}, hit{"/checkout", 1100})
hits.
	FoldedBy(
		func(h hit) string { return h.Path },
		func(string) int { return 0 },
		func(sum int, h hit) int { return sum + h.Bytes },
	).
	Filter(func(_ string, bytes int) bool { return bytes >= 1000 }).
	ForEach(func(path string, bytes int) { fmt.Println(path, bytes) })
```

```
/checkout 2000
/home 2400
```

## GroupedBy

```go
func GroupedBy[T any, K comparable](s Seq[T], sel func(T) K) Seq2[K, []T]
```

GroupedBy is GroupBy that stays in the chain: it yields each key with its bucket, keys in the order they were first seen and each bucket in encounter order. Every bucket is a fresh slice. Seq.FoldedBy is the form that keeps an accumulator per key instead of every element. A package function for the same instantiation- cycle reason as Chunked: Seq2[K, []T] leads back to Seq[[]T].

:::caution
Fully drains the source before the first emission and retains every element — hangs on infinite input
:::

```go
// GroupBy in first-seen key order, as a sequence of pairs rather
// than a map — so it can be ranged over and printed as it stands.
files := catena.Of("main.go", "README.md", "server.go", "deploy.yaml")
for ext, names := range catena.GroupedBy(files, filepath.Ext) {
	fmt.Println(ext, names)
}
```

```
.go [main.go server.go]
.md [README.md]
.yaml [deploy.yaml]
```

## GroupBy

```go
func (s Seq[T]) GroupBy[K comparable](sel func(T) K) map[K][]T
```

GroupBy collects elements into per-key buckets, each in encounter order.

:::caution
Full drain; retains every element; map iteration order is undefined
:::

```go
// Retains every element. For an aggregate, FoldBy is bounded by keys.
type order struct {
	Customer string
	ID       int
}
orders := catena.Of(order{"acme", 1001}, order{"globex", 1002}, order{"acme", 1003})
byCustomer := orders.GroupBy(func(o order) string { return o.Customer })
fmt.Println(len(byCustomer["acme"]), byCustomer["globex"])
```

```
2 [{globex 1002}]
```

## IndexBy

```go
func (s Seq[T]) IndexBy[K comparable](sel func(T) K) map[K]T
```

IndexBy maps each key to its element; on duplicate keys the last element wins.

:::caution
Full drain; map iteration order is undefined
:::

```go
// A lookup table; on a duplicate key the last element wins — which
// is what defaults followed by overrides want.
type setting struct{ Key, Value string }
config := catena.Of(
	setting{"port", "8080"}, setting{"log", "info"}, // defaults
	setting{"port", "9090"}, // override
).IndexBy(func(s setting) string { return s.Key })
fmt.Println(config["port"].Value, config["log"].Value)
```

```
9090 info
```

## TallyBy

```go
func (s Seq[T]) TallyBy[K comparable](sel func(T) K) map[K]int
```

TallyBy counts elements per key.

:::caution
Full drain; map iteration order is undefined
:::

```go
// A count per key, with no group ever built: responses by class.
statuses := catena.Of(200, 503, 201, 404, 500, 200)
byClass := statuses.TallyBy(func(code int) string { return fmt.Sprintf("%dxx", code/100) })
fmt.Println(byClass["2xx"], byClass["4xx"], byClass["5xx"])
```

```
3 1 2
```

## Associate

```go
func (s Seq[T]) Associate[K comparable, V any](f func(T) (K, V)) map[K]V
```

Associate builds a map from f's key/value pairs; on duplicate keys the last pair wins.

:::caution
Full drain; map iteration order is undefined
:::

```go
// Each element supplies both the key and the value.
env := catena.Of("HOME=/home/ada", "SHELL=/bin/zsh", "LANG=en_GB.UTF-8").
	Associate(func(line string) (string, string) {
		name, value, _ := strings.Cut(line, "=")
		return name, value
	})
fmt.Println(env["SHELL"], env["LANG"])
```

```
/bin/zsh en_GB.UTF-8
```

## Partition

```go
func (s Seq[T]) Partition(pred func(T) bool) (yes, no []T)
```

Partition splits elements by pred, preserving encounter order on both sides; nil slices for empty sides.

:::caution
Full drain
:::

```go
// Both sides in one pass, each in encounter order.
signups := catena.Of("ada@example.com", "bob", "eve@example.org", "n/a")
valid, rejected := signups.Partition(func(email string) bool {
	return strings.Contains(email, "@")
})
fmt.Println(valid, rejected)
```

```
[ada@example.com eve@example.org] [bob n/a]
```

## AssociateWith

```go
func AssociateWith[T comparable, V any](s Seq[T], f func(T) V) map[T]V
```

AssociateWith maps each element to f(element); on duplicate elements the last value wins.

:::caution
Full drain; map iteration order is undefined
:::

```go
// The elements become the keys; the function supplies each value.
hosts := catena.Of("web-1", "db-1")
probes := catena.AssociateWith(hosts, func(host string) string {
	return "https://" + host + ".internal/healthz"
})
fmt.Println(probes["db-1"])
```

```
https://db-1.internal/healthz
```

## Tally

```go
func Tally[T comparable](s Seq[T]) map[T]int
```

Tally counts occurrences per value.

:::caution
Full drain; map iteration order is undefined
:::

```go
// How many times each value occurs.
methods := catena.Of("GET", "POST", "GET", "GET", "DELETE")
counts := catena.Tally(methods)
fmt.Println(counts["GET"], counts["POST"], counts["PUT"])
```

```
3 1 0
```

## ToKeySet

```go
func ToKeySet[T comparable](s Seq[T]) map[T]struct{}
```

ToKeySet drains the sequence into a membership map, whose keys are the distinct elements. Named for what it returns rather than for the set it stands in for: ToSet is reserved for a real set type, should Go ever grow one.

:::caution
Full drain; map iteration order is undefined
:::

```go
// Duplicates collapse into a set built for membership tests.
blocked := catena.ToKeySet(catena.Of("10.0.0.7", "10.0.0.9", "10.0.0.7"))
_, denied := blocked["10.0.0.9"]
fmt.Println(len(blocked), denied)
```

```
2 true
```
