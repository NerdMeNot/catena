# Combining

Putting two or more sequences together, or pairing a sequence with itself. Concat and Chain are lazy in both operands; Zip pulls its second operand, and the set operations buffer theirs.

## Concat

```go
func (s Seq[T]) Concat(others ...Seq[T]) Seq[T]
```

Concat yields s, then each of the others in order.

```go
fmt.Println(catena.Of(1, 2).Concat(catena.Of(3), catena.Of(4, 5)).Collect())
```

```
[1 2 3 4 5]
```

## Append

```go
func (s Seq[T]) Append(vals ...T) Seq[T]
```

Append yields s, then the given values.

```go
fmt.Println(catena.Of("a").Append("b", "c").Collect())
```

```
[a b c]
```

## Prepend

```go
func (s Seq[T]) Prepend(vals ...T) Seq[T]
```

Prepend yields the given values, then s.

```go
fmt.Println(catena.Of("c").Prepend("a", "b").Collect())
```

```
[a b c]
```

## Chain

```go
func Chain[T any](seqs ...Seq[T]) Seq[T]
```

Chain yields each sequence's elements in order. It is the package-function form of Seq.Concat, for when you hold a slice of sequences and no receiver.

```go
// The variadic form, for when the sequences are in a slice already.
fmt.Println(catena.Chain(catena.Of(1), catena.Of(2), catena.Of(3)).Collect())
```

```
[1 2 3]
```

## Zip

```go
func (s Seq[T]) Zip[U any](other Seq[U]) Seq2[T, U]
```

Zip pairs elements of s with elements of other, stopping at the shorter side. The receiver drives; other is consumed through iter.Pull (its cleanup always runs). other is pulled once per emitted pair; the receiver is consumed one element past the pair count when other is shorter.

```go
// Pairs elements positionally and stops at the shorter side.
names := catena.Of("ada", "bob", "eve")
scores := catena.Of(90, 85)
fmt.Println(names.Zip(scores).
	MapTo(func(n string, s int) string { return fmt.Sprintf("%s=%d", n, s) }).
	Collect())
```

```
[ada=90 bob=85]
```

## ZipWithNext

```go
func (s Seq[T]) ZipWithNext() Seq2[T, T]
```

ZipWithNext yields each adjacent pair (element, next element). Empty and single-element input yield nothing.

```go
// Each element paired with its successor — deltas, gaps, transitions.
fmt.Println(catena.Of(3, 7, 12).ZipWithNext().
	MapTo(func(a, b int) int { return b - a }).
	Collect())
```

```
[4 5]
```

## WithIndex

```go
func (s Seq[T]) WithIndex() Seq2[int, T]
```

WithIndex pairs each element with its index, counting from 0.

```go
fmt.Println(catena.Of("a", "b").WithIndex().
	MapTo(func(i int, s string) string { return fmt.Sprintf("%d%s", i, s) }).
	Collect())
```

```
[0a 1b]
```

## JoinBy

```go
func (s Seq[T]) JoinBy[U any, K comparable, R any](
	other Seq[U],
	leftKey func(T) K, rightKey func(U) K,
	combine func(T, U) R,
) Seq[R]
```

JoinBy is a relational inner join: it pairs each element of s with every element of other sharing the same key and yields combine for each pair. It is unrelated to Join and JoinToString, which concatenate strings. Unmatched elements on either side are dropped; duplicate keys produce the cross product per key. Output order is left encounter order, then right encounter order within a key.

:::caution
Buffers all of other before the first emission
:::

```go
// A relational inner join: unmatched rows on either side are dropped,
// and duplicate keys produce the cross product.
type order struct {
	Customer int
	Amount   int
}
type customer struct {
	ID   int
	Name string
}
orders := catena.Of(order{1, 30}, order{2, 10}, order{9, 99})
customers := catena.Of(customer{1, "ada"}, customer{2, "bob"})

fmt.Println(orders.JoinBy(customers,
	func(o order) int { return o.Customer },
	func(c customer) int { return c.ID },
	func(o order, c customer) string { return fmt.Sprintf("%s:%d", c.Name, o.Amount) },
).Collect())
```

```
[ada:30 bob:10]
```

## Union

```go
func Union[T comparable](a, b Seq[T]) Seq[T]
```

Union yields the distinct elements of a, then the distinct elements of b not in a — set semantics, encounter order.

:::caution
Retains one entry per distinct value — unbounded
:::

```go
// Set semantics: the result is deduplicated, in encounter order,
// left operand first.
fmt.Println(catena.Union(catena.Of(1, 2, 2), catena.Of(3, 1)).Collect())
```

```
[1 2 3]
```

## Intersect

```go
func Intersect[T comparable](a, b Seq[T]) Seq[T]
```

Intersect yields the distinct elements of a that occur in b, in a's encounter order.

:::caution
Buffers all of b before a is consumed, plus a seen- set of a's distinct values
:::

```go
fmt.Println(catena.Intersect(catena.Of(1, 2, 3), catena.Of(3, 1)).Collect())
```

```
[1 3]
```

## Except

```go
func Except[T comparable](a, b Seq[T]) Seq[T]
```

Except yields the distinct elements of a that do not occur in b, in a's encounter order.

:::caution
Buffers all of b before a is consumed, plus a seen- set of a's distinct values
:::

```go
fmt.Println(catena.Except(catena.Of(1, 2, 3), catena.Of(2)).Collect())
```

```
[1 3]
```

## Intersperse

```go
func (s Seq[T]) Intersperse(sep T) Seq[T]
```

Intersperse yields sep between consecutive elements.

```go
fmt.Println(catena.Of("a", "b", "c").Intersperse("-").Collect())
```

```
[a - b - c]
```

## IfEmpty

```go
func (s Seq[T]) IfEmpty(defaults ...T) Seq[T]
```

IfEmpty yields s, or the given defaults if s yields nothing.

```go
// A fallback for the whole sequence, not per element.
fmt.Println(catena.Of(1, 2).IfEmpty(0).Collect())
fmt.Println(catena.Empty[int]().IfEmpty(0).Collect())
```

```
[1 2]
[0]
```
