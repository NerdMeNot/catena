---
title: "Streaming"
description: "infinite sequences, `Scan` running state, `Windowed` moving averages, `Chunked` batching"
sidebar:
  order: 6
---

Streaming: sequences with no end, and the operators that stay safe on them — running state with Scan, moving averages with Windowed, batching with Chunked, and change detection with DedupeBy. Every stage here is O(1) or bounded memory; nothing drains.

Run it: `go run ./examples/06-streaming`

```go
// Streaming: sequences with no end, and the operators that stay safe on
// them — running state with Scan, moving averages with Windowed, batching
// with Chunked, and change detection with DedupeBy. Every stage here is
// O(1) or bounded memory; nothing drains.
package main

import (
	"fmt"

	"github.com/NerdMeNot/catena"
)

type Reading struct {
	Tick int
	Temp float64
}

func main() {
	// An infinite sensor: a deterministic wobble around 20 degrees.
	sensor := catena.Generate(0, func(t int) int { return t + 1 }).
		Map(func(t int) Reading {
			wobble := float64((t*37)%17) - 8
			return Reading{t, 20 + wobble/4}
		})

	// Scan carries running state down the stream — here an exponential
	// moving average, emitted per reading. The initial value seeds the
	// fold but is not itself emitted.
	type Smoothed struct {
		Reading
		EMA float64
	}
	smoothed := sensor.Scan(Smoothed{EMA: 20}, func(s Smoothed, r Reading) Smoothed {
		return Smoothed{r, 0.8*s.EMA + 0.2*r.Temp}
	})
	fmt.Println("smoothed stream:")
	for s := range smoothed.Take(5) {
		fmt.Printf("  t=%d temp=%.2f ema=%.2f\n", s.Tick, s.Temp, s.EMA)
	}

	// Windowed gives overlapping views — a centered moving average over
	// five readings, advancing one at a time. Each window is a fresh
	// slice, so keeping one around is safe.
	temps := sensor.Map(func(r Reading) float64 { return r.Temp })
	fmt.Println("5-reading moving average:")
	for w := range catena.Windowed(temps, 5, 1).Take(4).Seq() {
		avg, _ := catena.Average(catena.FromSlice(w))
		fmt.Printf("  %.2f\n", avg)
	}

	// Chunked batches a stream for anything that wants groups — bulk
	// inserts, API calls with a page size.
	fmt.Println("batches of 4 ticks:")
	ticks := sensor.Map(func(r Reading) int { return r.Tick })
	for batch := range catena.Chunked(ticks, 4).Take(3).Seq() {
		fmt.Printf("  %v\n", batch)
	}

	// DedupeBy is the streaming-safe distinct: O(1) memory, collapsing
	// CONSECUTIVE repeats — exactly right for change detection on an
	// unbounded stream, where DistinctBy's seen-set would grow forever.
	fmt.Println("temperature zone changes:")
	zones := sensor.
		Map(func(r Reading) string {
			switch {
			case r.Temp < 19:
				return "cold"
			case r.Temp > 21:
				return "warm"
			default:
				return "ok"
			}
		}).
		DedupeBy(catena.Self[string]).
		Take(6).
		Collect()
	fmt.Printf("  %v\n", zones)

	// Cycle turns any finite sequence into a repeating one.
	shifts := catena.Cycle(catena.Of("day", "night")).Take(5).Collect()
	fmt.Println("shift rotation:", shifts)
}
```

[View on GitHub](https://github.com/NerdMeNot/catena/blob/main/examples/06-streaming/main.go)
