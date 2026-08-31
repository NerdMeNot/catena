# Eager

List is a []T carrying the whole Seq operation set, evaluated at once with exact preallocation. Only the List-only methods are listed here: the mirrored operators behave identically to the Seq ones over AsSeq(), which is not a claim but a conformance check, so their entries live on the pages above. Reach for List when the data is small and in memory, when you touch the result more than once, or when you want O(1) access.

## Len

```go
func (l List[T]) Len() int
```

Len returns the number of elements. O(1).

```go
l := catena.List[int]{3, 1, 2}
fmt.Println(l.Len())
```

```
3
```

## At

```go
func (l List[T]) At(i int) T
```

At returns the element at index i, panicking exactly like l[i] on an out-of-range index. O(1).

```go
// Panics exactly like l[i] — it IS the index expression.
l := catena.List[string]{"a", "b"}
fmt.Println(l.At(1))
```

```
b
```

## Get

```go
func (l List[T]) Get(i int) (T, bool)
```

Get returns the element at index i; (zero, false) for a negative or out-of-range index. O(1).

```go
// The comma-ok form, for when the index may be out of range.
l := catena.List[string]{"a"}
v, ok := l.Get(0)
_, bad := l.Get(9)
fmt.Println(v, ok, bad)
```

```
a true false
```

## Slice

```go
func (l List[T]) Slice(i, j int) List[T]
```

Slice returns l[i:j] as a List, panicking exactly like the slice expression — and, exactly like it, sharing the backing array.

```go
// It is the slice expression, aliasing included.
l := catena.List[int]{1, 2, 3, 4}
fmt.Println(l.Slice(1, 3))
```

```
[2 3]
```

## Clone

```go
func (l List[T]) Clone() List[T]
```

Clone returns a shallow copy with a fresh backing array.

```go
l := catena.List[int]{1, 2}
c := l.Clone()
c[0] = 99
fmt.Println(l, c)
```

```
[1 2] [99 2]
```

## AsSeq

```go
func (l List[T]) AsSeq() Seq[T]
```

AsSeq returns a lazy, re-iterable view of the list. The list is not copied; mutations are visible to later iterations.

```go
// Crossing to lazy is explicit, and free — it is a view, not a copy.
l := catena.List[int]{1, 2, 3, 4}
first, _ := l.AsSeq().Find(func(n int) bool { return n > 2 })
fmt.Println(first)
```

```
3
```

## FoldRight

```go
func (l List[T]) FoldRight[A any](init A, f func(T, A) A) A
```

FoldRight reduces right to left. It exists only on List: a right fold needs the whole sequence in memory, which a List already is (§7.5).

```go
// Exists only on List: a right fold needs the whole sequence, which
// a List already is.
l := catena.List[string]{"a", "b", "c"}
fmt.Println(l.FoldRight("|", func(s, acc string) string { return "(" + s + acc + ")" }))
```

```
(a(b(c|)))
```

## Append

```go
func (l List[T]) Append(vals ...T) List[T]
```

Append returns a new List with vals appended. Unlike the built-in append, the result ALWAYS has a fresh backing array and never aliases l — consistent with every other List transform. Callers who want the built-in's amortized behavior can use it directly: List is a []T.

```go
// Unlike the builtin, the result never aliases the receiver — every
// List transform returns fresh backing memory.
base := make(catena.List[int], 2, 10)
grown := base.Append(9)
grown[0] = 99
fmt.Println(base, grown)
```

```
[0 0] [99 0 9]
```
