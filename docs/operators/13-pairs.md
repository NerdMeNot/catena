# Pairs

Seq2 is the standard library's pairing type with methods — what maps.All and slices.All speak, so every boundary with the standard library is a free conversion. It is deliberately a bridge rather than a peer: it carries what you need to get back to Seq and little else, and MapTo is the intended exit.

## Filter

```go
func (s Seq2[K, V]) Filter(pred func(K, V) bool) Seq2[K, V]
```

Filter yields the pairs pred admits.

```go
pairs := catena.Of(1, 2, 3, 4).WithIndex().
	Filter(func(i, v int) bool { return v%2 == 0 })
fmt.Println(pairs.Values().Collect())
```

```
[2 4]
```

## FilterNot

```go
func (s Seq2[K, V]) FilterNot(pred func(K, V) bool) Seq2[K, V]
```

FilterNot yields the pairs pred rejects.

```go
pairs := catena.Of(1, 2, 3).WithIndex().
	FilterNot(func(i, v int) bool { return v == 2 })
fmt.Println(pairs.Values().Collect())
```

```
[1 3]
```

## Map

```go
func (s Seq2[K, V]) Map[K2, V2 any](f func(K, V) (K2, V2)) Seq2[K2, V2]
```

Map yields f applied to each pair.

```go
pairs := catena.Of("a", "b").WithIndex().
	Map(func(i int, s string) (string, int) { return s, i * 10 })
fmt.Println(catena.CollectMap(pairs))
```

```
map[a:0 b:10]
```

## MapValues

```go
func (s Seq2[K, V]) MapValues[V2 any](f func(K, V) V2) Seq2[K, V2]
```

MapValues yields each pair with its value replaced by f(k, v). f receives the key too (Kotlin-consistent).

```go
// The callback receives the key as well as the value.
pairs := catena.Of("a", "b").WithIndex().
	MapValues(func(i int, s string) string { return fmt.Sprintf("%d%s", i, s) })
fmt.Println(pairs.Values().Collect())
```

```
[0a 1b]
```

## MapTo

```go
func (s Seq2[K, V]) MapTo[U any](f func(K, V) U) Seq[U]
```

MapTo collapses each pair into one value — the intended exit back to Seq and its full API.

```go
// The intended exit: collapse each pair into one value and continue
// in Seq, where the full API lives.
fmt.Println(catena.Of("a", "b").WithIndex().
	MapTo(func(i int, s string) string { return fmt.Sprintf("%d%s", i, s) }).
	Collect())
```

```
[0a 1b]
```

## Take

```go
func (s Seq2[K, V]) Take(n int) Seq2[K, V]
```

Take yields at most the first n pairs. Panics if n is negative.

```go
fmt.Println(catena.Range(1, 9, 1).WithIndex().Take(3).Values().Collect())
```

```
[1 2 3]
```

## Drop

```go
func (s Seq2[K, V]) Drop(n int) Seq2[K, V]
```

Drop skips the first n pairs. Panics if n is negative.

```go
fmt.Println(catena.Range(1, 5, 1).WithIndex().Drop(2).Values().Collect())
```

```
[3 4]
```

## Keys

```go
func (s Seq2[K, V]) Keys() Seq[K]
```

Keys yields the first element of each pair. Calling Keys and Values on the same single-pass Seq2 is a double consume — use Unzip.

```go
// Keys and Values on the SAME single-pass Seq2 is a double consume;
// Unzip does both in one pass.
fmt.Println(catena.Of("a", "b").WithIndex().Keys().Collect())
```

```
[0 1]
```

## Values

```go
func (s Seq2[K, V]) Values() Seq[V]
```

Values yields the second element of each pair.

```go
fmt.Println(catena.Of("a", "b").WithIndex().Values().Collect())
```

```
[a b]
```

## Swap

```go
func (s Seq2[K, V]) Swap() Seq2[V, K]
```

Swap yields each pair with its sides exchanged.

```go
pairs := catena.Of("a", "b").WithIndex().Swap()
fmt.Println(pairs.Keys().Collect())
```

```
[a b]
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
total := catena.Of(10, 20, 30).WithIndex().
	Fold(0, func(acc, i, v int) int { return acc + i*v })
fmt.Println(total)
```

```
80
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
catena.Of("a", "b").WithIndex().ForEach(func(i int, s string) {
	fmt.Printf("%d=%s ", i, s)
})
```

```
0=a 1=b
```

## Any

```go
func (s Seq2[K, V]) Any(pred func(K, V) bool) bool
```

Any reports whether pred admits any pair; stops at the first match.

```go
fmt.Println(catena.Of(1, 2).WithIndex().Any(func(i, v int) bool { return v == 2 }))
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
fmt.Println(catena.Of(2, 4).WithIndex().All(func(i, v int) bool { return v%2 == 0 }))
```

```
true
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
fmt.Println(catena.Of("a", "b", "c").WithIndex().Count())
```

```
3
```

## First

```go
func (s Seq2[K, V]) First() (K, V, bool)
```

First returns the first pair.

```go
i, v, ok := catena.Of("a", "b").WithIndex().First()
fmt.Println(i, v, ok)
```

```
0 a true
```

## Pull

```go
func (s Seq2[K, V]) Pull() (next func() (K, V, bool), stop func())
```

Pull converts s to a pull-based iterator. THE CALLER MUST CALL stop, even if next has returned false, or resources held by s will leak.

```go
next, stop := catena.Of("a", "b").WithIndex().Pull()
defer stop()
i, v, ok := next()
fmt.Println(i, v, ok)
```

```
0 a true
```

## Seq2

```go
func (s Seq2[K, V]) Seq2() iter.Seq2[K, V]
```

Seq2 converts to the stdlib iterator type. Free.

```go
for i, v := range catena.Of("a", "b").WithIndex().Seq2() {
	fmt.Printf("%d=%s ", i, v)
}
```

```
0=a 1=b
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
// To a map. On a duplicate key the last value wins, as with plain
// map assignment.
fmt.Println(catena.CollectMap(catena.Of("a", "b").WithIndex()))
```

```
map[0:a 1:b]
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
// a single-use Seq2.
idx, vals := catena.Unzip(catena.Of("a", "b").WithIndex())
fmt.Println(idx, vals)
```

```
[0 1] [a b]
```
