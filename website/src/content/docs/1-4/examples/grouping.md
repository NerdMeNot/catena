---
title: "Grouping"
description: "`FoldBy` with struct accumulators, composite keys, `TallyBy`, `IndexBy`"
sidebar:
  order: 2
---

Grouping: FoldBy is the library's centerpiece — streaming aggregation bounded by distinct keys, not elements. This example builds up from a count to a full multi-field statistics struct, then to composite keys.

Run it: `go run ./examples/02-grouping`

```go
// Grouping: FoldBy is the library's centerpiece — streaming aggregation
// bounded by distinct keys, not elements. This example builds up from a
// count to a full multi-field statistics struct, then to composite keys.
package main

import (
	"fmt"
	"maps"
	"slices"

	"github.com/NerdMeNot/catena"
)

type Sale struct {
	Region  string
	Product string
	Amount  int
}

var sales = []Sale{
	{"west", "widget", 120}, {"east", "widget", 80}, {"west", "gadget", 300},
	{"east", "gadget", 150}, {"west", "widget", 60}, {"east", "widget", 220},
	{"west", "gadget", 90}, {"east", "gadget", 40}, {"west", "widget", 500},
}

func main() {
	s := catena.FromSlice(sales)

	// Level 1 — counting is TallyBy.
	perRegion := s.TallyBy(func(x Sale) string { return x.Region })
	fmt.Println("sales per region:", perRegion)

	// Level 2 — summing one field is FoldBy with an int accumulator.
	revenue := s.FoldBy(
		func(x Sale) string { return x.Region },
		func(string) int { return 0 },
		func(sum int, x Sale) int { return sum + x.Amount },
	)
	fmt.Println("revenue per region:", revenue)

	// Level 3 — the accumulator can be any struct, so one pass computes
	// count, sum, min, and max together. GroupBy would retain every Sale
	// to do this; FoldBy retains one Stats per region.
	type Stats struct {
		Count, Sum, Min, Max int
	}
	stats := s.FoldBy(
		func(x Sale) string { return x.Region },
		func(string) Stats { return Stats{Min: int(^uint(0) >> 1)} },
		func(st Stats, x Sale) Stats {
			st.Count++
			st.Sum += x.Amount
			st.Min = min(st.Min, x.Amount)
			st.Max = max(st.Max, x.Amount)
			return st
		},
	)
	for _, region := range slices.Sorted(maps.Keys(stats)) {
		st := stats[region]
		fmt.Printf("%s: n=%d sum=%d min=%d max=%d avg=%.0f\n",
			region, st.Count, st.Sum, st.Min, st.Max, float64(st.Sum)/float64(st.Count))
	}

	// Level 4 — a composite key is just a comparable struct. No string
	// concatenation tricks, no nested maps.
	type Key struct{ Region, Product string }
	byBoth := s.FoldBy(
		func(x Sale) Key { return Key{x.Region, x.Product} },
		func(Key) int { return 0 },
		func(sum int, x Sale) int { return sum + x.Amount },
	)
	fmt.Println("west/gadget revenue:", byBoth[Key{"west", "gadget"}])

	// IndexBy when you want a lookup table instead of an aggregate:
	// last-wins on duplicate keys, like plain map assignment.
	biggestSeen := catena.FromSlice(sales).
		SortedBy(func(x Sale) int { return x.Amount }).
		IndexBy(func(x Sale) string { return x.Region })
	fmt.Println("largest sale per region:", biggestSeen)
}
```

[View on GitHub](https://github.com/NerdMeNot/catena/blob/main/examples/02-grouping/main.go)
