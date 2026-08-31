---
title: "Filtering"
description: "Keeping some elements and dropping others — by predicate, by key, or by what has already been seen. Everything here streams: no operator on this page buffers the input, though the distinct family retains one entry per distinct key."
sidebar:
  order: 1
---

Keeping some elements and dropping others — by predicate, by key, or by what has already been seen. Everything here streams: no operator on this page buffers the input, though the distinct family retains one entry per distinct key.

## Filter

```go
func (s Seq[T]) Filter(pred func(T) bool) Seq[T]
```

Filter yields the elements for which pred returns true.

```go
fmt.Println(catena.Of(1, 2, 3, 4, 5).
	Filter(func(n int) bool { return n%2 == 0 }).
	Collect())
```

```
[2 4]
```

## FilterNot

```go
func (s Seq[T]) FilterNot(pred func(T) bool) Seq[T]
```

FilterNot yields the elements for which pred returns false.

```go
// The negated form, for when the predicate reads better positively.
fmt.Println(catena.Of("go", "", "rust", "", "zig").
	FilterNot(func(s string) bool { return s == "" }).
	Collect())
```

```
[go rust zig]
```

## FilterIndexed

```go
func (s Seq[T]) FilterIndexed(pred func(int, T) bool) Seq[T]
```

FilterIndexed yields the elements for which pred(index, element) returns true. The index counts source elements from 0.

```go
// The index counts source elements, not surviving ones.
fmt.Println(catena.Of("a", "b", "c", "d", "e").
	FilterIndexed(func(i int, _ string) bool { return i%2 == 0 }).
	Collect())
```

```
[a c e]
```

## FilterErr

```go
func (s Seq[T]) FilterErr(pred func(T) (bool, error)) Try[T]
```

FilterErr yields elements pred admits, as a Try; a failed pred call yields (zero, err).

```go
// A predicate that can fail produces a Try, so the caller chooses
// what a failure means rather than the pipeline deciding.
ports := catena.Of("80", "443", "https", "8080")
valid := ports.FilterErr(func(s string) (bool, error) {
	if strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyz") {
		return false, fmt.Errorf("not a port: %q", s)
	}
	return len(s) > 2, nil
})

kept, errs := valid.CollectAll()
fmt.Println(kept, errs)
```

```
[443 8080] [not a port: "https"]
```

## Distinct

```go
func Distinct[T comparable](s Seq[T]) Seq[T]
```

Distinct yields elements not seen before; the first occurrence wins.

:::caution
Retains one entry per distinct value — unbounded
:::

```go
// Distinct constrains the element type, so it is a package function
// rather than a method — a method on Seq[T any] may require nothing
// of T. First occurrence wins, and encounter order is preserved.
fmt.Println(catena.Distinct(catena.Of(3, 1, 3, 2, 1)).Collect())
```

```
[3 1 2]
```

## DistinctBy

```go
func (s Seq[T]) DistinctBy[K comparable](sel func(T) K) Seq[T]
```

DistinctBy yields elements whose key has not been seen before; the first occurrence wins.

:::caution
Retains one key per distinct value — unbounded
:::

```go
type user struct {
	Org, Name string
}
users := catena.Of(
	user{"acme", "ada"},
	user{"globex", "bob"},
	user{"acme", "eve"},
)

// One user per org; the first occurrence wins.
fmt.Println(users.
	DistinctBy(func(u user) string { return u.Org }).
	Collect())
```

```
[{acme ada} {globex bob}]
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
// For keys that are not comparable, or an equality of your own.
// Retains every distinct element and compares against all of them,
// so this is for small inputs.
fmt.Println(catena.Of("Go", "GO", "rust", "go").
	DistinctWith(strings.EqualFold).
	Collect())
```

```
[Go rust]
```

## Dedupe

```go
func Dedupe[T comparable](s Seq[T]) Seq[T]
```

Dedupe yields elements that differ from their predecessor — consecutive duplicates only, O(1) memory.

```go
// Consecutive duplicates only. On sorted input it equals Distinct at
// a fraction of the cost; on unsorted input the two differ.
fmt.Println(catena.Dedupe(catena.Of(3, 3, 1, 1, 3)).Collect())
```

```
[3 1 3]
```

## DedupeBy

```go
func (s Seq[T]) DedupeBy[K comparable](sel func(T) K) Seq[T]
```

DedupeBy yields elements whose key differs from the previous element's key — consecutive duplicates only, O(1) memory. On key-sorted input it equals DistinctBy at a fraction of the cost.

```go
// Collapses CONSECUTIVE runs only, in O(1) memory — the streaming
// alternative to DistinctBy, and the right choice on an unbounded
// source where a seen-set would grow forever.
type reading struct {
	Tick int
	Zone string
}
readings := catena.Of(
	reading{1, "cold"}, reading{2, "cold"},
	reading{3, "warm"}, reading{4, "cold"},
)

fmt.Println(readings.
	DedupeBy(func(r reading) string { return r.Zone }).
	Collect())
```

```
[{1 cold} {3 warm} {4 cold}]
```

## NonZero

```go
func NonZero[T comparable](s Seq[T]) Seq[T]
```

NonZero yields the elements that are not the zero value of T.

```go
// Drops the zero value of T — empty strings here, but equally 0,
// nil pointers, or a zero struct.
fmt.Println(catena.NonZero(catena.Of("go", "", "rust", "")).Collect())
```

```
[go rust]
```
