---
title: "Pairs"
description: "Seq2 is the standard library's pairing type with methods — what maps.All and slices.All speak, so every boundary with the standard library is a free conversion. It is deliberately a bridge rather than a peer: it carries what you need to get back to Seq and little else, and MapTo is the intended exit."
sidebar:
  order: 13
---

Seq2 is the standard library's pairing type with methods — what maps.All and slices.All speak, so every boundary with the standard library is a free conversion. It is deliberately a bridge rather than a peer: it carries what you need to get back to Seq and little else, and MapTo is the intended exit.

## Filter

```go
func (s Seq2[K, V]) Filter(pred func(K, V) bool) Seq2[K, V]
```

Filter yields the pairs pred admits.

```go
// The predicate sees both sides of the pair: pick hosts by their
// measured latency.
latencies := catena.Of("api-1", "api-2", "db-1").Zip(catena.Of(42, 310, 275))
slow := latencies.Filter(func(host string, ms int) bool { return ms > 200 })
fmt.Println(slow.Keys().Collect())
```

```
[api-2 db-1]
```

## FilterNot

```go
func (s Seq2[K, V]) FilterNot(pred func(K, V) bool) Seq2[K, V]
```

FilterNot yields the pairs pred rejects.

```go
// Drop the comment lines of a config file while every surviving line
// keeps its original number, ready for an error message.
lines := catena.Of("# cache", "ttl=60", "# auth", "realm=prod").WithIndex().
	FilterNot(func(_ int, line string) bool { return strings.HasPrefix(line, "#") })
lines.ForEach(func(i int, line string) { fmt.Printf("line %d: %s\n", i+1, line) })
```

```
line 2: ttl=60
line 4: realm=prod
```

## Map

```go
func (s Seq2[K, V]) Map[K2, V2 any](f func(K, V) (K2, V2)) Seq2[K2, V2]
```

Map yields f applied to each pair.

```go
// Both sides can change, types included: a ranked list becomes a
// name-to-place lookup.
finishers := catena.Of("ada", "grace", "linus").WithIndex()
places := finishers.Map(func(i int, name string) (string, int) { return name, i + 1 })
fmt.Println(catena.CollectMap(places))
```

```
map[ada:1 grace:2 linus:3]
```

## MapValues

```go
func (s Seq2[K, V]) MapValues[V2 any](f func(K, V) V2) Seq2[K, V2]
```

MapValues yields each pair with its value replaced by f(k, v). f receives the key too (Kotlin-consistent).

```go
// Keys stay put and values are rewritten. The callback receives the
// key as well, so the new value can depend on it — here, which items
// are on sale.
onSale := map[string]bool{"mug": true}
prices := catena.Of("mug", "tee").Zip(catena.Of(1200, 2500)).
	MapValues(func(sku string, cents int) int {
		if onSale[sku] {
			return cents / 2
		}
		return cents
	})
fmt.Println(catena.CollectMap(prices))
```

```
map[mug:600 tee:2500]
```

## MapTo

```go
func (s Seq2[K, V]) MapTo[U any](f func(K, V) U) Seq[U]
```

MapTo collapses each pair into one value — the intended exit back to Seq and its full API.

```go
// The intended exit: collapse each pair into one value and continue
// in Seq, where the full API lives.
hosts := catena.Of("api.internal", "db.internal")
addrs := hosts.Zip(catena.Of("443", "5432")).MapTo(net.JoinHostPort)
fmt.Println(addrs.Collect())
```

```
[api.internal:443 db.internal:5432]
```

## Take

```go
func (s Seq2[K, V]) Take(n int) Seq2[K, V]
```

Take yields at most the first n pairs. Panics if n is negative.

```go
// The podium: the first three finishers, each still paired with its
// position.
finishers := catena.Of("ada", "grace", "linus", "ken", "dennis").WithIndex()
finishers.Take(3).ForEach(func(i int, name string) {
	fmt.Printf("%d. %s\n", i+1, name)
})
```

```
1. ada
2. grace
3. linus
```

## Drop

```go
func (s Seq2[K, V]) Drop(n int) Seq2[K, V]
```

Drop skips the first n pairs. Panics if n is negative.

```go
// Skip a CSV header after numbering the lines, so the data rows keep
// the line numbers a user would see in their editor.
rows := catena.Of("sku,qty", "mug,4", "tee,9").WithIndex().Drop(1)
rows.ForEach(func(i int, row string) { fmt.Printf("line %d: %s\n", i+1, row) })
```

```
line 2: mug,4
line 3: tee,9
```

## Keys

```go
func (s Seq2[K, V]) Keys() Seq[K]
```

Keys yields the first element of each pair. Calling Keys and Values on the same single-pass Seq2 is a double consume — use Unzip.

```go
// The settings a config map defines, in a stable order. Keys and
// Values on the SAME single-pass Seq2 is a double consume; Unzip does
// both in one pass.
config := map[string]string{"port": "8080", "host": "localhost", "debug": "true"}
fmt.Println(catena.FromMap(config).Keys().Then(catena.Sorted).Collect())
```

```
[debug host port]
```

## Values

```go
func (s Seq2[K, V]) Values() Seq[V]
```

Values yields the second element of each pair.

```go
// Leave the keys behind once only the values matter: total units in
// stock, whatever the SKU.
stock := map[string]int{"mug": 4, "tee": 9, "cap": 0}
fmt.Println(catena.FromMap(stock).Values().Then(catena.Sum))
```

```
13
```

## Swap

```go
func (s Seq2[K, V]) Swap() Seq2[V, K]
```

Swap yields each pair with its sides exchanged.

```go
// Invert a lookup table: from code-to-name to name-to-code.
names := map[int]string{200: "OK", 404: "Not Found"}
codes := catena.CollectMap(catena.FromMap(names).Swap())
fmt.Println(codes["Not Found"], codes)
```

```
404 map[Not Found:404 OK:200]
```

## Fold

```go
func (s Seq2[K, V]) Fold[A any](init A, f func(A, K, V) A) A
```

Fold reduces the pairs into an accumulator, left to right.

:::caution
Full drain
:::

```go
// A cart total: each pair is a quantity and a unit price in cents.
cart := catena.Of(2, 1, 3).Zip(catena.Of(1200, 2500, 400))
total := cart.Fold(0, func(sum, qty, cents int) int { return sum + qty*cents })
fmt.Println(total)
```

```
6100
```

## ForEach

```go
func (s Seq2[K, V]) ForEach(f func(K, V))
```

ForEach calls f on every pair.

:::caution
Full drain
:::

```go
// Both sides arrive as separate arguments — no pair type to unpack.
env := catena.Of("HOST", "PORT").Zip(catena.Of("localhost", "8080"))
env.ForEach(func(name, value string) {
	fmt.Printf("%s=%s\n", name, value)
})
```

```
HOST=localhost
PORT=8080
```

## Any

```go
func (s Seq2[K, V]) Any(pred func(K, V) bool) bool
```

Any reports whether pred admits any pair; stops at the first match.

```go
// Is anything out of stock? Stops at the first pair that says yes.
stock := map[string]int{"mug": 4, "tee": 9, "cap": 0}
fmt.Println(catena.FromMap(stock).Any(func(_ string, units int) bool { return units == 0 }))
```

```
true
```

## All

```go
func (s Seq2[K, V]) All(pred func(K, V) bool) bool
```

All reports whether pred admits every pair; stops at the first counterexample. Vacuously true on empty input.

```go
// Is every required setting filled in? Stops at the first one that is
// not.
settings := map[string]string{"host": "localhost", "port": "8080", "token": ""}
complete := catena.FromMap(settings).All(func(_, value string) bool { return value != "" })
fmt.Println(complete)
```

```
false
```

## Count

```go
func (s Seq2[K, V]) Count() int
```

Count returns the number of pairs.

:::caution
Full drain
:::

```go
// How many SKUs are running low.
stock := map[string]int{"mug": 4, "tee": 9, "cap": 0}
low := catena.FromMap(stock).Filter(func(_ string, units int) bool { return units < 5 })
fmt.Println(low.Count())
```

```
2
```

## First

```go
func (s Seq2[K, V]) First() (K, V, bool)
```

First returns the first pair.

```go
// The first item that cannot be fulfilled, with the comma-ok form for
// when there is none. Nothing after it is examined.
stock := catena.Of("mug", "cap", "tee").Zip(catena.Of(4, 0, 0))
sku, units, ok := stock.Filter(func(_ string, units int) bool { return units == 0 }).First()
fmt.Println(sku, units, ok)
```

```
cap 0 true
```

## Pull

```go
func (s Seq2[K, V]) Pull() (next func() (K, V, bool), stop func())
```

Pull converts s to a pull-based iterator. THE CALLER MUST CALL stop, even if next has returned false, or resources held by s will leak.

```go
// When the consumer has to drive: take one pair now and the rest
// later. Ownership transfers to you: always call stop.
fields := catena.Of("id", "email").Zip(catena.Of("42", "ada@example.com"))
next, stop := fields.Pull()
defer stop()
column, value, ok := next()
fmt.Println(column, value, ok)
```

```
id 42 true
```

## Seq2

```go
func (s Seq2[K, V]) Seq2() iter.Seq2[K, V]
```

Seq2 converts to the stdlib iterator type. Free.

```go
// The standard iterator type, for the stdlib functions that take one —
// here maps.Insert, layering overrides onto defaults.
config := map[string]string{"host": "localhost", "port": "8080"}
overrides := catena.Of("port", "debug").Zip(catena.Of("9090", "true"))
maps.Insert(config, overrides.Seq2())
fmt.Println(config)
```

```
map[debug:true host:localhost port:9090]
```

## CollectMap

```go
func CollectMap[K comparable, V any](s Seq2[K, V]) map[K]V
```

CollectMap drains a pair sequence into a map; on duplicate keys the last value wins.

:::caution
Full drain; map iteration order is undefined
:::

```go
// To a map. On a duplicate key the last value wins, as with plain map
// assignment — so a later setting overrides an earlier one.
names := catena.Of("port", "host", "port")
settings := catena.CollectMap(names.Zip(catena.Of("8080", "localhost", "9090")))
fmt.Println(settings)
```

```
map[host:localhost port:9090]
```

## Unzip

```go
func Unzip[K, V any](s Seq2[K, V]) ([]K, []V)
```

Unzip drains a pair sequence into its two sides; nil slices for empty input.

:::caution
Full drain; buffers both sides
:::

```go
// Both sides in one pass — the safe way to get keys and values from
// a single-use Seq2. Here: which log lines failed, and what they said.
log := catena.Of("ok boot", "ERROR disk full", "ok sync", "ERROR timeout")
failures := log.WithIndex().
	Filter(func(_ int, line string) bool { return strings.HasPrefix(line, "ERROR") })
lineNumbers, messages := catena.Unzip(failures)
fmt.Println(lineNumbers, messages)
```

```
[1 3] [ERROR disk full ERROR timeout]
```
