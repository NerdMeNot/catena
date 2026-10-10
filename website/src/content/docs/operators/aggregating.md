---
title: "Aggregating"
description: "Extremes, totals and joins. Two conventions run through this page: -By returns the element with the extreme key while -Of returns the key itself, and ties always go to the first element seen. NaN orders below everything, so these agree with the sorts. TopNBy is the one to reach for on a large scan — a bounded heap rather than a full sort."
sidebar:
  order: 11
---

Extremes, totals and joins. Two conventions run through this page: -By returns the element with the extreme key while -Of returns the key itself, and ties always go to the first element seen. NaN orders below everything, so these agree with the sorts. TopNBy is the one to reach for on a large scan — a bounded heap rather than a full sort.

## MaxBy

```go
func (s Seq[T]) MaxBy[K cmp.Ordered](sel func(T) K) (T, bool)
```

MaxBy returns the element with the largest key; the earliest maximal element wins ties. NaN keys order below everything (cmp.Compare). MaxOf returns the key itself; catena.Max(s) is the no-selector form.

:::caution
Full drain
:::

```go
// Returns the ELEMENT with the largest key; ties go to the first.
type endpoint struct {
	Path  string
	P99ms int
}
endpoints := catena.Of(endpoint{"/home", 120}, endpoint{"/search", 840}, endpoint{"/export", 840})
slowest, _ := endpoints.MaxBy(func(e endpoint) int { return e.P99ms })
fmt.Println(slowest.Path)
```

```
/search
```

## MinBy

```go
func (s Seq[T]) MinBy[K cmp.Ordered](sel func(T) K) (T, bool)
```

MinBy returns the element with the smallest key; the earliest minimal element wins ties. MinOf returns the key itself; catena.Min(s) is the no-selector form.

:::caution
Full drain
:::

```go
// The whole quote comes back, not just its price.
type quote struct {
	Carrier string
	Cents   int
}
quotes := catena.Of(quote{"dhl", 1250}, quote{"ups", 990}, quote{"fedex", 1400})
cheapest, _ := quotes.MinBy(func(q quote) int { return q.Cents })
fmt.Println(cheapest.Carrier)
```

```
ups
```

## MaxOf

```go
func (s Seq[T]) MaxOf[K cmp.Ordered](sel func(T) K) (K, bool)
```

MaxOf returns the largest key — MaxBy returns the element that carries it.

:::caution
Full drain
:::

```go
// Returns the KEY, where MaxBy returns the element — the -By/-Of
// distinction, which holds across the whole library.
type file struct {
	Name  string
	Bytes int64
}
files := catena.Of(file{"app.log", 52_000}, file{"core.dump", 9_800_000}, file{"go.mod", 310})
largest, _ := files.MaxOf(func(f file) int64 { return f.Bytes })
fmt.Println(largest)
```

```
9800000
```

## MinOf

```go
func (s Seq[T]) MinOf[K cmp.Ordered](sel func(T) K) (K, bool)
```

MinOf returns the smallest key — MinBy returns the element that carries it.

:::caution
Full drain
:::

```go
// The lowest stock level, whichever product it belongs to.
type product struct {
	SKU     string
	InStock int
}
shelf := catena.Of(product{"mug", 14}, product{"tee", 3}, product{"cap", 27})
lowest, _ := shelf.MinOf(func(p product) int { return p.InStock })
fmt.Println(lowest)
```

```
3
```

## MinMaxOf

```go
func (s Seq[T]) MinMaxOf[K cmp.Ordered](sel func(T) K) (min, max K, ok bool)
```

MinMaxOf returns the smallest and largest keys in one pass.

:::caution
Full drain
:::

```go
// Both ends in a single pass.
type reading struct {
	Sensor string
	TempC  float64
}
readings := catena.Of(reading{"attic", 31.5}, reading{"cellar", 12.0}, reading{"hall", 20.5})
coldest, warmest, ok := readings.MinMaxOf(func(r reading) float64 { return r.TempC })
fmt.Println(coldest, warmest, ok)
```

```
12 31.5 true
```

## MaxWith

```go
func (s Seq[T]) MaxWith(cmp func(a, b T) int) (T, bool)
```

MaxWith returns the largest element under cmp; the earliest maximal element wins ties.

:::caution
Full drain
:::

```go
// A comparator, for orderings no single key expresses: major first,
// then minor.
type version struct{ Major, Minor int }
tags := catena.Of(version{1, 12}, version{2, 3}, version{2, 0})
latest, _ := tags.MaxWith(func(a, b version) int {
	return cmp.Or(cmp.Compare(a.Major, b.Major), cmp.Compare(a.Minor, b.Minor))
})
fmt.Println(latest)
```

```
{2 3}
```

## MinWith

```go
func (s Seq[T]) MinWith(cmp func(a, b T) int) (T, bool)
```

MinWith returns the smallest element under cmp; the earliest minimal element wins ties.

:::caution
Full drain
:::

```go
// What to work on next: the most urgent ticket, and among equally
// urgent ones the oldest.
type ticket struct {
	ID       string
	Priority int // 1 is most urgent
	AgeDays  int
}
backlog := catena.Of(ticket{"T-7", 2, 30}, ticket{"T-8", 1, 2}, ticket{"T-9", 1, 6})
next, _ := backlog.MinWith(func(a, b ticket) int {
	return cmp.Or(cmp.Compare(a.Priority, b.Priority), cmp.Compare(b.AgeDays, a.AgeDays))
})
fmt.Println(next.ID)
```

```
T-9
```

## TopNBy

```go
func (s Seq[T]) TopNBy[K cmp.Ordered](n int, sel func(T) K) []T
```

TopNBy returns the n elements with the largest keys, sorted descending by key; equal keys retain encounter order and the earliest elements win at the cut. Memory is O(n) — the streaming alternative to SortedByDesc().Take(n). catena.TopN(s, n) is the no-selector form, and BottomNBy is the same operation at the other end. Panics if n is negative.

:::caution
Full drain
:::

```go
// A bounded heap of n, not a sort: memory is O(n) rather than O(all),
// which is the difference between a working pipeline and an OOM on a
// large scan. Output is sorted descending, ties in encounter order.
type customer struct {
	Name  string
	Spend int
}
customers := catena.Of(
	customer{"acme", 5200}, customer{"initech", 800}, customer{"globex", 9100},
	customer{"hooli", 5200}, customer{"umbrella", 300},
)
fmt.Println(customers.TopNBy(3, func(c customer) int { return c.Spend }))
```

```
[{globex 9100} {acme 5200} {hooli 5200}]
```

## BottomNBy

```go
func (s Seq[T]) BottomNBy[K cmp.Ordered](n int, sel func(T) K) []T
```

BottomNBy returns the n elements with the smallest keys, sorted ascending by key; equal keys retain encounter order. Memory is O(n) — the streaming alternative to SortedBy().Take(n). catena.BottomN(s, n) is the no-selector form. Panics if n is negative.

:::caution
Full drain
:::

```go
// The same bounded heap at the other end, sorted ascending: the two
// least-loaded servers to send new work to.
type server struct {
	Host string
	Load int
}
pool := catena.Of(server{"web-1", 71}, server{"web-2", 12}, server{"web-3", 94}, server{"web-4", 35})
fmt.Println(pool.BottomNBy(2, func(s server) int { return s.Load }))
```

```
[{web-2 12} {web-4 35}]
```

## Sample

```go
func (s Seq[T]) Sample(n int, rng *rand.Rand) []T
```

Sample returns n elements chosen at random, every set of n being equally likely, in encounter order; all of them if the sequence has n or fewer. Memory is O(n) — reservoir sampling, the streaming alternative to collecting everything and shuffling. rng supplies the randomness, so a seeded one makes the choice repeatable; nil uses math/rand/v2's shared generator. Panics if n is negative.

:::caution
Full drain
:::

```go
// Five of a million, each equally likely, holding five in memory.
// A seeded generator makes the choice repeatable; pass nil for the
// shared one.
rng := rand.New(rand.NewPCG(1, 2))
fmt.Println(len(catena.Range(0, 1_000_000, 1).Sample(5, rng)))

// A short sequence is returned whole, in order.
fmt.Println(catena.Of("a", "b").Sample(5, rng))
```

```
5
[a b]
```

## Max

```go
func Max[T cmp.Ordered](s Seq[T]) (T, bool)
```

Max returns the largest element; NaN orders below everything (cmp.Compare).

:::caution
Full drain
:::

```go
// The no-selector form of MaxOf, for values that order themselves.
latenciesMs := catena.Of(120, 85, 310, 96)
worst, ok := catena.Max(latenciesMs)
fmt.Println(worst, ok)
```

```
310 true
```

## Min

```go
func Min[T cmp.Ordered](s Seq[T]) (T, bool)
```

Min returns the smallest element; NaN orders below everything, so a NaN in the input is the minimum.

:::caution
Full drain
:::

```go
// False on empty, since there is no smallest of nothing.
prices := catena.Of(12.50, 9.99, 14.00)
cheapest, ok := catena.Min(prices)
_, nonEmpty := catena.Min(catena.Empty[float64]())
fmt.Println(cheapest, ok, nonEmpty)
```

```
9.99 true false
```

## MinMax

```go
func MinMax[T cmp.Ordered](s Seq[T]) (min, max T, ok bool)
```

MinMax returns the smallest and largest elements in one pass.

:::caution
Full drain
:::

```go
// The range of a day's readings in one pass rather than two.
tempsC := catena.Of(14.5, 21.0, 9.5, 17.0)
low, high, ok := catena.MinMax(tempsC)
fmt.Println(low, high, ok)
```

```
9.5 21 true
```

## TopN

```go
func TopN[T cmp.Ordered](s Seq[T], n int) []T
```

TopN returns the n largest elements, sorted descending; equal elements retain encounter order. Memory is O(n) — the streaming alternative to SortedDesc(s).Take(n). Panics if n is negative.

:::caution
Full drain
:::

```go
// The podium, best first, without sorting the whole field.
scores := catena.Of(71, 94, 58, 88, 90)
fmt.Println(catena.TopN(scores, 3))
```

```
[94 90 88]
```

## BottomN

```go
func BottomN[T cmp.Ordered](s Seq[T], n int) []T
```

BottomN returns the n smallest elements, sorted ascending; equal elements retain encounter order. Memory is O(n) — the streaming alternative to Sorted(s).Take(n). Panics if n is negative.

:::caution
Full drain
:::

```go
// The two fastest laps, quickest first.
lapSecs := catena.Of(92.4, 89.7, 95.1, 90.2)
fmt.Println(catena.BottomN(lapSecs, 2))
```

```
[89.7 90.2]
```

## SumOf

```go
func (s Seq[T]) SumOf[N Numeric](sel func(T) N) N
```

SumOf sums the selected values; integer overflow wraps like +. Empty input sums to 0. catena.Sum(s) is the no-selector form.

:::caution
Full drain
:::

```go
// The selector computes what to add, so a basket totals without an
// intermediate Map.
type line struct {
	Qty, UnitCents int
}
basket := catena.Of(line{2, 1250}, line{1, 400}, line{3, 99})
fmt.Println(basket.SumOf(func(l line) int { return l.Qty * l.UnitCents }))
```

```
3197
```

## ProductOf

```go
func (s Seq[T]) ProductOf[N Numeric](sel func(T) N) N
```

ProductOf multiplies the selected values. Empty input yields 1, the multiplicative identity.

:::caution
Full drain
:::

```go
// A request that needs every dependency is up only as often as all
// of them are. The empty product is 1, the multiplicative identity:
// nothing to depend on, nothing to fail.
type dependency struct {
	Name   string
	Uptime float64
}
uptime := func(d dependency) float64 { return d.Uptime }
deps := catena.Of(dependency{"db", 0.999}, dependency{"cache", 0.995}, dependency{"auth", 0.99})
fmt.Printf("%.3f\n", deps.ProductOf(uptime))
fmt.Printf("%.3f\n", catena.Empty[dependency]().ProductOf(uptime))
```

```
0.984
1.000
```

## AverageOf

```go
func (s Seq[T]) AverageOf[N Numeric](sel func(T) N) (float64, bool)
```

AverageOf returns the mean of the selected values, accumulating in float64 (naive summation — precision for large integer inputs is not guaranteed); (0, false) on empty input.

:::caution
Full drain
:::

```go
// Accumulates in float64, so integer latencies give a fractional
// mean — and reports false on empty rather than dividing by zero.
type request struct {
	Path string
	Ms   int
}
ms := func(r request) int { return r.Ms }
requests := catena.Of(request{"/home", 120}, request{"/cart", 85}, request{"/search", 310})
mean, ok := requests.AverageOf(ms)
_, nonEmpty := catena.Empty[request]().AverageOf(ms)
fmt.Printf("%.2f %v %v\n", mean, ok, nonEmpty)
```

```
171.67 true false
```

## Sum

```go
func Sum[T Numeric](s Seq[T]) T
```

Sum adds the elements; integer overflow wraps like +. Empty input sums to 0.

:::caution
Full drain
:::

```go
// Integer overflow wraps, exactly as + does.
downloadsPerDay := catena.Of(1_204, 980, 1_532)
fmt.Println(catena.Sum(downloadsPerDay))
```

```
3716
```

## Product

```go
func Product[T Numeric](s Seq[T]) T
```

Product multiplies the elements. Empty input yields 1, the multiplicative identity.

:::caution
Full drain
:::

```go
// A parcel's volume from its sides, in cubic centimetres.
sidesCm := catena.Of(30, 20, 15)
fmt.Println(catena.Product(sidesCm))
```

```
9000
```

## Average

```go
func Average[T Numeric](s Seq[T]) (float64, bool)
```

Average returns the mean, accumulating in float64; (0, false) on empty input.

:::caution
Full drain
:::

```go
// Always a float64, whatever the element type; false on empty.
ratings := catena.Of(5, 4, 4, 2)
mean, ok := catena.Average(ratings)
fmt.Println(mean, ok)
```

```
3.75 true
```

## JoinToString

```go
func (s Seq[T]) JoinToString(sep string, sel func(T) string) string
```

JoinToString concatenates the selected strings with sep between elements.

:::caution
Full drain
:::

```go
// Map and join in one step: the To line of an email.
type user struct{ Name, Email string }
team := catena.Of(user{"Ada", "ada@example.com"}, user{"Bob", "bob@example.com"})
fmt.Println(team.JoinToString(", ", func(u user) string {
	return fmt.Sprintf("%s <%s>", u.Name, u.Email)
}))
```

```
Ada <ada@example.com>, Bob <bob@example.com>
```

## Join

```go
func Join(s Seq[string], sep string) string
```

Join concatenates a string sequence with sep between elements.

:::caution
Full drain
:::

```go
// strings.Join for a sequence, with no slice in between.
segments := catena.Of("api", "v2", "orders")
fmt.Println("/" + catena.Join(segments, "/"))
```

```
/api/v2/orders
```
