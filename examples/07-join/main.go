// Join: JoinBy is a relational inner join between two streams of structs
// — the right side is indexed by key, the left side streams past it, and
// the joined stream is a plain Seq you keep chaining on. This example
// joins orders to customers, rolls the joined stream up by country, then
// shows the two joins that drop nothing: LeftJoinBy and GroupJoinBy.
package main

import (
	"fmt"

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

	// The joined stream is ordinary, so aggregation composes directly.
	// FoldedBy keeps the result a stream as well — one pair per country,
	// in the order the countries were first seen — so the rollup can be
	// ranked without leaving the chain.
	type Revenue struct {
		Country string
		Total   int
	}
	ranked := enriched.
		FoldedBy(
			func(e Enriched) string { return e.Customer.Country },
			func(string) int { return 0 },
			func(sum int, e Enriched) int { return sum + e.Order.Amount },
		).
		MapTo(func(country string, total int) Revenue { return Revenue{country, total} }).
		SortedByDesc(func(r Revenue) int { return r.Total })
	for r := range ranked {
		fmt.Printf("%s revenue: %d\n", r.Country, r.Total)
	}

	// An inner join dropped order 105. LeftJoinBy keeps every left row
	// and tells combine whether it found a match.
	fmt.Println("every order, matched or not:")
	for line := range catena.FromSlice(orders).LeftJoinBy(
		catena.FromSlice(customers),
		func(o OrderRow) int { return o.Customer },
		func(c Customer) int { return c.ID },
		func(o OrderRow, c Customer, ok bool) string {
			if !ok {
				return fmt.Sprintf("  order %d: no such customer %d", o.ID, o.Customer)
			}
			return fmt.Sprintf("  order %d: %s", o.ID, c.Name)
		},
	) {
		fmt.Println(line)
	}

	// GroupJoinBy is the one-to-many join that keeps the many together:
	// one row per customer with all of their orders, where JoinBy would
	// repeat the customer once per order. A customer with no orders would
	// still get a row, with none.
	fmt.Println("orders per customer:")
	for line := range catena.FromSlice(customers).GroupJoinBy(
		catena.FromSlice(orders),
		func(c Customer) int { return c.ID },
		func(o OrderRow) int { return o.Customer },
		func(c Customer, os []OrderRow) string {
			return fmt.Sprintf("  %s: %d orders, %d total", c.Name, len(os),
				catena.FromSlice(os).SumOf(func(o OrderRow) int { return o.Amount }))
		},
	) {
		fmt.Println(line)
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
