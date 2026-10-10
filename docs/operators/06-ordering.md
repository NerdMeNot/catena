# Ordering

Sorting and reversing. Every sort here is stable, and every one buffers the whole sequence — they cannot emit a first element until they have seen the last, so none of them terminates on an infinite source.

## Sorted

```go
func Sorted[T cmp.Ordered](s Seq[T]) Seq[T]
```

Sorted yields the elements in ascending order, stably. NaN sorts first.

:::caution
Buffers the entire input
:::

```go
// Ascending order for any ordered type. A map hands its keys back in a
// different order on every run; sorting them makes the output stable.
env := map[string]string{"PORT": "8080", "HOME": "/srv/app", "LOG_LEVEL": "info"}
fmt.Println(catena.Sorted(catena.FromMap(env).Keys()).Collect())
```

```
[HOME LOG_LEVEL PORT]
```

## SortedDesc

```go
func SortedDesc[T cmp.Ordered](s Seq[T]) Seq[T]
```

SortedDesc yields the elements in descending order, stably.

:::caution
Buffers the entire input
:::

```go
// Largest first: the slowest responses, in milliseconds, at the top
// of the report.
latencies := catena.Of(87, 912, 45, 340)
fmt.Println(catena.SortedDesc(latencies).Collect())
```

```
[912 340 87 45]
```

## SortedBy

```go
func (s Seq[T]) SortedBy[K cmp.Ordered](sel func(T) K) Seq[T]
```

SortedBy yields the elements sorted ascending by key, stably. sel is called exactly once per element (decorate-sort-undecorate). catena.Sorted(s) is the no-selector form, for an ordered T.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
// Stable, and the selector runs exactly once per element rather than
// once per comparison — so an expensive key is affordable. Stability
// shows here: tickets of equal priority stay in the order they arrived.
type ticket struct {
	ID       string
	Priority int
}
queue := catena.Of(ticket{"T-1", 2}, ticket{"T-2", 1}, ticket{"T-3", 2}, ticket{"T-4", 1})
fmt.Println(queue.SortedBy(func(t ticket) int { return t.Priority }).Collect())
```

```
[{T-2 1} {T-4 1} {T-1 2} {T-3 2}]
```

## SortedByDesc

```go
func (s Seq[T]) SortedByDesc[K cmp.Ordered](sel func(T) K) Seq[T]
```

SortedByDesc yields the elements sorted descending by key, stably. sel is called exactly once per element.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
// Largest key first: what is filling the disk.
type file struct {
	Name  string
	Bytes int
}
files := catena.Of(file{"app.log", 9_400}, file{"core.dump", 880_000}, file{"cache.db", 61_000})
fmt.Println(files.SortedByDesc(func(f file) int { return f.Bytes }).Collect())
```

```
[{core.dump 880000} {cache.db 61000} {app.log 9400}]
```

## SortedWith

```go
func (s Seq[T]) SortedWith(cmp func(a, b T) int) Seq[T]
```

SortedWith yields the elements sorted by cmp, stably.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
// A comparator, for orderings a single key cannot express: by region,
// then busiest first within each region.
type host struct {
	Region string
	Name   string
	Load   int
}
hosts := catena.Of(host{"us", "us-1", 40}, host{"eu", "eu-1", 15}, host{"us", "us-2", 85}, host{"eu", "eu-2", 60})
fmt.Println(hosts.
	SortedWith(func(a, b host) int {
		return cmp.Or(strings.Compare(a.Region, b.Region), cmp.Compare(b.Load, a.Load))
	}).
	Map(func(h host) string { return h.Name }).
	Collect())
```

```
[eu-2 eu-1 us-2 us-1]
```

## Reversed

```go
func (s Seq[T]) Reversed() Seq[T]
```

Reversed yields the elements in reverse order.

:::caution
Buffers the entire input — hangs on infinite input
:::

```go
// Last in, first out: migrations are rolled back in the opposite
// order to the one they were applied in.
applied := catena.Of("001_users", "002_orders", "003_add_index")
fmt.Println(applied.Reversed().Collect())
```

```
[003_add_index 002_orders 001_users]
```

## IsSorted

```go
func IsSorted[T cmp.Ordered](s Seq[T]) bool
```

IsSorted reports whether the elements are in ascending order; an empty or one-element sequence is sorted. NaN orders below everything (cmp.Compare), as it does for Sorted. It stops at the first pair out of order, so on infinite input it returns only if there is one.

```go
// One pass, no buffer, and it stops at the first pair out of order.
// Equal neighbours count as sorted: two events in the same second are
// fine, a timestamp that goes backwards is not.
steady := catena.Of(1700000001, 1700000004, 1700000004, 1700000009)
skewed := catena.Of(1700000001, 1700000009, 1700000004)
fmt.Println(catena.IsSorted(steady), catena.IsSorted(skewed))
```

```
true false
```

## IsSortedBy

```go
func (s Seq[T]) IsSortedBy[K cmp.Ordered](sel func(T) K) bool
```

IsSortedBy reports whether no element's key orders before its predecessor's; an empty or one-element sequence is sorted. sel is called once per element. It stops at the first pair out of order, so on infinite input it returns only if there is one.

```go
// Checks an ordering by one field without sorting anything: is the
// changelog oldest-first? ISO dates compare correctly as strings.
type release struct {
	Tag  string
	Date string
}
changelog := catena.Of(
	release{"v1.0.0", "2025-11-03"},
	release{"v1.1.0", "2026-01-19"},
	release{"v1.0.1", "2026-01-12"},
)
fmt.Println(changelog.IsSortedBy(func(r release) string { return r.Date }))
```

```
false
```

## IsSortedWith

```go
func (s Seq[T]) IsSortedWith(cmp func(a, b T) int) bool
```

IsSortedWith reports whether no element orders before its predecessor under cmp; an empty or one-element sequence is sorted. It stops at the first pair out of order, so on infinite input it returns only if there is one. catena.IsSorted is the no-comparator form, for an ordered T.

```go
// Any ordering a comparator can express: here, highest score first.
// A tie is still in order.
type player struct {
	Name  string
	Score int
}
board := catena.Of(player{"ada", 90}, player{"bob", 85}, player{"eve", 85}, player{"dan", 70})
fmt.Println(board.IsSortedWith(func(a, b player) int { return cmp.Compare(b.Score, a.Score) }))
```

```
true
```
