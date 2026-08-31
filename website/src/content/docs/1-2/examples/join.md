---
title: "Join"
description: "`JoinBy`: relational joins between struct tables, then rollups on the joined stream"
sidebar:
  order: 7
---

Join: JoinBy is a relational inner join between two streams of structs — the right side is indexed by key, the left side streams past it, and the joined stream is a plain Seq you keep chaining on. This example joins orders to customers, then rolls the joined stream up by country.

Run it: `go run ./examples/07-join`

```go
// Join: JoinBy is a relational inner join between two streams of structs
// — the right side is indexed by key, the left side streams past it, and
// the joined stream is a plain Seq you keep chaining on. This example
// joins orders to customers, then rolls the joined stream up by country.
package main

import (
	"fmt"
	"maps"
	"slices"

	"github.com/NerdMeNot/catena"
)

type Customer struct {
	ID      int
	Name    string
	Country string
}

type OrderRow struct {
	ID       int
	Customer int
	Amount   int
}

var customers = []Customer{
	{1, "ada", "NL"}, {2, "bob", "US"}, {3, "eve", "NL"}, {4, "kim", "JP"},
}

var orders = []OrderRow{
	{101, 1, 250}, {102, 2, 90}, {103, 1, 40}, {104, 3, 700},
	{105, 9, 999}, // customer 9 does not exist: dropped by the inner join
	{106, 4, 120}, {107, 2, 310},
}

type Enriched struct {
	Order    OrderRow
	Customer Customer
}

func main() {
	// The join itself: left stream, right stream, a key selector for
	// each side, and how to combine a matching pair.
	enriched := catena.FromSlice(orders).JoinBy(
		catena.FromSlice(customers),
		func(o OrderRow) int { return o.Customer },
		func(c Customer) int { return c.ID },
		func(o OrderRow, c Customer) Enriched { return Enriched{o, c} },
	)

	fmt.Println("joined rows:")
	for e := range enriched {
		fmt.Printf("  order %d: %s (%s) — %d\n",
			e.Order.ID, e.Customer.Name, e.Customer.Country, e.Order.Amount)
	}
	// Note order 105 is gone: no matching customer, and an inner join
	// drops unmatched rows on both sides.

	// The joined stream is ordinary, so aggregation composes directly:
	// revenue per country, one pass over the join.
	revenue := enriched.FoldBy(
		func(e Enriched) string { return e.Customer.Country },
		func(string) int { return 0 },
		func(sum int, e Enriched) int { return sum + e.Order.Amount },
	)
	for _, country := range slices.Sorted(maps.Keys(revenue)) {
		fmt.Printf("%s revenue: %d\n", country, revenue[country])
	}

	// Join key multiplicity is the relational one: duplicate keys on the
	// right produce a cross product per key. Two tags per order here.
	type Tag struct {
		Customer int
		Label    string
	}
	tags := catena.Of(
		Tag{1, "vip"}, Tag{1, "early-adopter"}, Tag{2, "trial"},
	)
	labeled := catena.FromSlice(orders).JoinBy(
		tags,
		func(o OrderRow) int { return o.Customer },
		func(t Tag) int { return t.Customer },
		func(o OrderRow, t Tag) string { return fmt.Sprintf("order %d is %s", o.ID, t.Label) },
	).Collect()
	fmt.Println("tagged:", len(labeled), "pairings")
	for _, l := range labeled {
		fmt.Println(" ", l)
	}
}
```

[View on GitHub](https://github.com/NerdMeNot/catena/blob/main/examples/07-join/main.go)
