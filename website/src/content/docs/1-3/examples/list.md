---
title: "List"
description: "eager `List`, crossing between eager and lazy, the `Seq2` bridge"
sidebar:
  order: 8
---

List and the Seq2 bridge: when the data is small and already in memory, eager evaluation with exact preallocation beats a lazy chain — and the two worlds convert explicitly, in both directions.

Run it: `go run ./examples/08-list`

```go
// List and the Seq2 bridge: when the data is small and already in memory,
// eager evaluation with exact preallocation beats a lazy chain — and the
// two worlds convert explicitly, in both directions.
package main

import (
	"fmt"
	"strings"

	"github.com/NerdMeNot/catena"
)

func main() {
	// A List IS a []T — construct it like one, index it like one.
	words := catena.List[string]{"the", "chain", "holds", "when", "every", "link", "does"}
	fmt.Println("len:", words.Len(), "third word:", words.At(2))

	// The whole Seq operation set exists eagerly. Each stage returns a
	// finished List (Map preallocates exactly), and the semantics are
	// conformance-checked to match the lazy version over AsSeq().
	shouting := words.
		Filter(func(w string) bool { return len(w) > 3 }).
		Map(strings.ToUpper)
	fmt.Println("emphasis:", shouting)

	// FoldRight exists only on List — a right fold needs the whole
	// sequence anyway, and a List already is one.
	nested := words.FoldRight("∎", func(w, acc string) string {
		return "(" + w + " " + acc + ")"
	})
	fmt.Println("right-nested:", nested)

	// Append never aliases: the result always has a fresh backing array,
	// like every other List transform — unlike the builtin.
	base := catena.List[int]{1, 2, 3}
	grown := base.Append(4, 5)
	grown[0] = 99
	fmt.Println("base untouched:", base, "grown:", grown)

	// Cross to lazy when short-circuiting starts to matter…
	firstLong, _ := words.AsSeq().Find(func(w string) bool { return len(w) >= 5 })
	fmt.Println("first long word:", firstLong)

	// …and back to eager when you want a materialized result.
	lengths := words.AsSeq().
		Map(func(w string) int { return len(w) }).
		ToList()
	fmt.Println("lengths:", lengths)

	// Seq2 is the bridge to pair-shaped data: maps in, pairs along,
	// and either a Seq (MapTo) or a map (CollectMap) out.
	tally := catena.FromSlice(lengths).TallyBy(catena.Self[int])
	report := catena.FromMap(tally).
		Filter(func(length, count int) bool { return count > 1 }).
		MapTo(func(length, count int) string {
			return fmt.Sprintf("%d words of length %d", count, length)
		}).
		Collect()
	fmt.Println("common lengths:", report)

	// WithIndex pairs elements with positions; Unzip splits a pair
	// sequence back into its two sides in one pass.
	idx, vals := catena.Unzip(words.AsSeq().Take(3).WithIndex())
	fmt.Println("positions:", idx, "values:", vals)
}
```

[View on GitHub](https://github.com/NerdMeNot/catena/blob/main/examples/08-list/main.go)
