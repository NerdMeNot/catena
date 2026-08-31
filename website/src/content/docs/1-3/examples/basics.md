---
title: "Basics"
description: "building pipelines, laziness, early termination, `range` interop"
sidebar:
  order: 1
---

Basics: building a pipeline, what laziness means in practice, and how catena sequences interoperate with plain range loops and iter.Seq.

Run it: `go run ./examples/01-basics`

```go
// Basics: building a pipeline, what laziness means in practice, and how
// catena sequences interoperate with plain range loops and iter.Seq.
package main

import (
	"fmt"
	"slices"

	"github.com/NerdMeNot/catena"
)

func main() {
	// A pipeline is built stage by stage and runs only when consumed.
	squares := catena.Range(1, 100, 1).
		Filter(func(n int) bool { return n%3 == 0 }).
		Map(func(n int) int { return n * n }).
		Take(4).
		Collect()
	fmt.Println("first four squares of multiples of 3:", squares)

	// Laziness is observable: this counts how many elements the source
	// actually produced. Take(3) means three — not a hundred.
	produced := 0
	counted := catena.From(func(yield func(int) bool) {
		for i := 1; i <= 100; i++ {
			produced++
			if !yield(i) {
				return
			}
		}
	})
	firstThree := counted.Take(3).Collect()
	fmt.Printf("took %v, source produced %d elements\n", firstThree, produced)

	// Seq IS iter.Seq: range over it directly, no conversion ceremony.
	total := 0
	for v := range catena.Of(10, 20, 30).Filter(func(v int) bool { return v > 10 }) {
		total += v
	}
	fmt.Println("ranged total:", total)

	// And the stdlib's iterators adapt for free in both directions.
	fromStdlib := catena.From(slices.Values([]string{"a", "b", "c"}))
	backToStdlib := slices.Collect(fromStdlib.Map(func(s string) string { return s + "!" }).Seq())
	fmt.Println("round trip through the stdlib:", backToStdlib)

	// Chains restructure without rewriting: each stage is a line.
	report := catena.Of("chain", "of", "sequences", "linked", "as", "one").
		FilterNot(func(w string) bool { return len(w) < 3 }).
		MapIndexed(func(i int, w string) string { return fmt.Sprintf("%d:%s", i, w) }).
		JoinToString(" ", catena.Self[string])
	fmt.Println("indexed words:", report)

	// Eager vs lazy, made visible. The same two stages run in a
	// different ORDER depending on the type. Lazy (Seq) interleaves:
	// each element flows through both stages before the next element
	// starts, and no intermediate collection ever exists.
	fmt.Println("\nlazy evaluation order (per element, interleaved):")
	catena.Of(1, 2, 3).
		OnEach(func(v int) { fmt.Printf("  stage A sees %d\n", v) }).
		OnEach(func(v int) { fmt.Printf("  stage B sees %d\n", v) }).
		Drain()

	// Eager (List) stages: stage A finishes the WHOLE collection —
	// allocating it — before stage B starts.
	fmt.Println("eager evaluation order (per stage, staged):")
	catena.List[int]{1, 2, 3}.
		OnEach(func(v int) { fmt.Printf("  stage A sees %d\n", v) }).
		OnEach(func(v int) { fmt.Printf("  stage B sees %d\n", v) }).
		Drain()
	// Same elements, same results, different shape of execution — which
	// is the whole eager/lazy distinction. When to prefer which:
	// docs/02-concepts.md, "Eager vs lazy — the actual difference".
}
```

[View on GitHub](https://github.com/NerdMeNot/catena/blob/main/examples/01-basics/main.go)
