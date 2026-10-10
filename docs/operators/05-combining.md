# Combining

Putting two or more sequences together, or pairing a sequence with itself. Concat and Chain are lazy in both operands; Zip pulls its second operand, and the set operations buffer theirs.

## Concat

```go
func (s Seq[T]) Concat(others ...Seq[T]) Seq[T]
```

Concat yields s, then each of the others in order.

```go
// One sequence after another, any number of them: resolvers in the
// order they should be tried. No sequence is read until the one before
// it is exhausted.
primary := catena.Of("10.0.0.2", "10.0.0.3")
office := catena.Of("192.168.1.1")
public := catena.Of("1.1.1.1", "8.8.8.8")
fmt.Println(primary.Concat(office, public).Collect())
```

```
[10.0.0.2 10.0.0.3 192.168.1.1 1.1.1.1 8.8.8.8]
```

## Append

```go
func (s Seq[T]) Append(vals ...T) Seq[T]
```

Append yields s, then the given values.

```go
// Loose values on the end, without wrapping them in a sequence first:
// the flags a wrapper always adds after the user's own arguments.
userArgs := catena.Of("test", "./...")
fmt.Println(userArgs.Append("-race", "-count=1").Collect())
```

```
[test ./... -race -count=1]
```

## Prepend

```go
func (s Seq[T]) Prepend(vals ...T) Seq[T]
```

Prepend yields the given values, then s.

```go
// Loose values on the front: a header row ahead of the data, without
// building a second sequence just to hold it.
rows := catena.Of("report.pdf,48200", "logo.png,9100")
for line := range rows.Prepend("name,bytes").Seq() {
	fmt.Println(line)
}
```

```
name,bytes
report.pdf,48200
logo.png,9100
```

## Chain

```go
func Chain[T any](seqs ...Seq[T]) Seq[T]
```

Chain yields each sequence's elements in order. It is the package-function form of Seq.Concat, for when you hold a slice of sequences and no receiver.

```go
// The variadic form of Concat, for when the sequences are in a slice
// already and none of them is the obvious receiver — one per rotated
// log file, say.
rotated := []catena.Seq[string]{
	catena.Of("mon:start", "mon:stop"),
	catena.Of("tue:start"),
	catena.Of("wed:start", "wed:crash"),
}
fmt.Println(catena.Chain(rotated...).Collect())
```

```
[mon:start mon:stop tue:start wed:start wed:crash]
```

## Zip

```go
func (s Seq[T]) Zip[U any](other Seq[U]) Seq2[T, U]
```

Zip pairs elements of s with elements of other, stopping at the shorter side. The receiver drives; other is consumed through iter.Pull (its cleanup always runs). other is pulled once per emitted pair; the receiver is consumed one element past the pair count when other is shorter.

```go
// Pairs elements positionally and stops at the shorter side: a CSV row
// that is missing its last field yields only the columns it has.
header := catena.Of("host", "port", "region")
row := catena.Of("db-1", "5432")
fmt.Println(header.Zip(row).
	MapTo(func(column, value string) string { return column + "=" + value }).
	Collect())
```

```
[host=db-1 port=5432]
```

## ZipWithNext

```go
func (s Seq[T]) ZipWithNext() Seq2[T, T]
```

ZipWithNext yields each adjacent pair (element, next element). Empty and single-element input yield nothing.

```go
// Each element paired with its successor — deltas, gaps, transitions.
// Here, the seconds between heartbeats; the long gap is the outage.
// Four heartbeats make three gaps.
heartbeats := catena.Of(0, 30, 61, 245)
fmt.Println(heartbeats.ZipWithNext().
	MapTo(func(prev, next int) int { return next - prev }).
	Collect())
```

```
[30 31 184]
```

## WithIndex

```go
func (s Seq[T]) WithIndex() Seq2[int, T]
```

WithIndex pairs each element with its index, counting from 0.

```go
// The index becomes half of a pair, so it survives later stages: the
// positions reported are those in the original file, counting from 0,
// not those in the filtered result.
lines := catena.Of("package main", "// TODO: add flags", "func main() {", "// TODO: exit code", "}")
fmt.Println(lines.WithIndex().
	Filter(func(_ int, line string) bool { return strings.Contains(line, "TODO") }).
	Keys().
	Collect())
```

```
[1 3]
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
// A relational inner join. Rows without a match are dropped on both
// sides — the order for unknown customer 9, and eve, who ordered
// nothing — and a key that matches twice produces a row each time.
type order struct {
	Customer int
	Amount   int
}
type customer struct {
	ID   int
	Name string
}
orders := catena.Of(order{1, 30}, order{2, 10}, order{9, 99}, order{1, 45})
customers := catena.Of(customer{1, "ada"}, customer{2, "bob"}, customer{3, "eve"})

fmt.Println(orders.JoinBy(customers,
	func(o order) int { return o.Customer },
	func(c customer) int { return c.ID },
	func(o order, c customer) string { return fmt.Sprintf("%s:%d", c.Name, o.Amount) },
).Collect())
```

```
[ada:30 bob:10 ada:45]
```

## LeftJoinBy

```go
func (s Seq[T]) LeftJoinBy[U any, K comparable, R any](
	other Seq[U],
	leftKey func(T) K, rightKey func(U) K,
	combine func(T, U, bool) R,
) Seq[R]
```

LeftJoinBy is a relational left outer join: like JoinBy, but a left element with no match is kept, and combine is called for it once with the zero U and false. Matched elements call combine with true once per match, so duplicate keys produce the cross product per key. Output order is left encounter order, then right encounter order within a key.

:::caution
Buffers all of other before the first emission
:::

```go
// A left outer join: the order with no customer on file is kept, and
// combine is told so, receiving a zero customer and false.
type order struct {
	Customer int
	Amount   int
}
type customer struct {
	ID   int
	Name string
}
orders := catena.Of(order{1, 30}, order{9, 99})
customers := catena.Of(customer{1, "ada"})

fmt.Println(orders.LeftJoinBy(customers,
	func(o order) int { return o.Customer },
	func(c customer) int { return c.ID },
	func(o order, c customer, ok bool) string {
		if !ok {
			return fmt.Sprintf("guest:%d", o.Amount)
		}
		return fmt.Sprintf("%s:%d", c.Name, o.Amount)
	},
).Collect())
```

```
[ada:30 guest:99]
```

## GroupJoinBy

```go
func (s Seq[T]) GroupJoinBy[U any, K comparable, R any](
	other Seq[U],
	leftKey func(T) K, rightKey func(U) K,
	combine func(T, []U) R,
) Seq[R]
```

GroupJoinBy pairs each element of s with all the elements of other sharing its key, yielding combine exactly once per left element — the one-to-many join that keeps the many together. matches is in right encounter order and nil when nothing matches. Its elements are shared by every left element with that key: retain it and append to it freely, but do not assign to its elements.

:::caution
Buffers all of other before the first emission
:::

```go
// One row per customer with all of their orders together — where
// JoinBy would repeat ada once per order and drop bob, who has none.
type order struct {
	Customer int
	Amount   int
}
type customer struct {
	ID   int
	Name string
}
customers := catena.Of(customer{1, "ada"}, customer{2, "bob"})
orders := catena.Of(order{1, 30}, order{1, 12}, order{9, 99})

fmt.Println(customers.GroupJoinBy(orders,
	func(c customer) int { return c.ID },
	func(o order) int { return o.Customer },
	func(c customer, os []order) string { return fmt.Sprintf("%s:%d", c.Name, len(os)) },
).Collect())
```

```
[ada:2 bob:0]
```

## Then

```go
func (s Seq[T]) Then[U any](f func(Seq[T]) U) U
```

Then returns f(s): it lets a function that takes a sequence sit inside a chain instead of wrapping it. That is every package function of one argument — s.Then(catena.Distinct).Then(catena.Sorted), or s.Then(catena.Sum) to finish — and every stage built by the parallel package. A function needing more arguments goes in a closure.

```go
// Package functions sit inside the chain instead of wrapping it, so
// the steps read in the order they happen: dedupe the hosts, sort
// them, roll out two at a time. A function taking more than the
// sequence goes in a closure.
hosts := catena.Of("web-2", "web-1", "web-2", "web-3", "web-1")
fmt.Println(hosts.
	Then(catena.Distinct).
	Then(catena.Sorted).
	Then(func(s catena.Seq[string]) catena.Seq[[]string] { return catena.Chunked(s, 2) }).
	Collect())
```

```
[[web-1 web-2] [web-3]]
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
// Set semantics: a permission granted twice, or by both the role and
// the team, appears once. Encounter order, left operand first.
fromRole := catena.Of("read", "write", "write")
fromTeam := catena.Of("deploy", "read")
fmt.Println(catena.Union(fromRole, fromTeam).Collect())
```

```
[read write deploy]
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
// What both sides have — the users active last week who came back
// this week — in the order of the first operand.
lastWeek := catena.Of("ada", "bob", "eve", "dan")
thisWeek := catena.Of("eve", "zoe", "ada")
fmt.Println(catena.Intersect(lastWeek, thisWeek).Collect())
```

```
[ada eve]
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
// What the first side has that the second does not: the hosts still
// waiting for a patch.
fleet := catena.Of("web-1", "web-2", "db-1", "cache-1")
patched := catena.Of("web-2", "cache-1")
fmt.Println(catena.Except(fleet, patched).Collect())
```

```
[web-1 db-1]
```

## Intersperse

```go
func (s Seq[T]) Intersperse(sep T) Seq[T]
```

Intersperse yields sep between consecutive elements.

```go
// A separator between elements but not before the first or after the
// last — the fence-post problem, handled once.
menu := catena.Of("Open", "Save", "Quit")
for line := range menu.Intersperse("----").Seq() {
	fmt.Println(line)
}
```

```
Open
----
Save
----
Quit
```

## IfEmpty

```go
func (s Seq[T]) IfEmpty(defaults ...T) Seq[T]
```

IfEmpty yields s, or the given defaults if s yields nothing.

```go
// A fallback for the whole sequence, not per element: the default
// applies only when nothing at all was configured.
configured := catena.Of("eu-1.example.com", "eu-2.example.com")
unset := catena.Empty[string]()
fmt.Println(configured.IfEmpty("localhost").Collect())
fmt.Println(unset.IfEmpty("localhost").Collect())
```

```
[eu-1.example.com eu-2.example.com]
[localhost]
```
