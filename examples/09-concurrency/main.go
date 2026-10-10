// Concurrency: the par package runs slow callbacks several at a time
// without the pipeline changing shape. A stage goes into the chain with
// Then; a failed call is an element, so the consumer still picks the
// error policy; and every goroutine a stage starts has exited before the
// line that consumed it returns.
package main

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync/atomic"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/par"
)

type Page struct {
	URL   string
	Bytes int
}

// fetch stands in for a network call: 50ms of latency, and one URL that
// always fails.
func fetch(ctx context.Context, url string) (Page, error) {
	select {
	case <-time.After(50 * time.Millisecond):
	case <-ctx.Done(): // a callback must honour its context
		return Page{}, ctx.Err()
	}
	if url == "https://down.example" {
		return Page{}, errors.New("connection refused: " + url)
	}
	return Page{url, 1000 + len(url)}, nil
}

func main() {
	ctx := context.Background()
	urls := []string{
		"https://a.example", "http://insecure.example", "https://b.example",
		"https://c.example", "https://down.example", "https://d.example",
		"https://e.example", "https://f.example", "https://g.example",
	}
	https := func(u string) bool { return len(u) > 5 && u[:5] == "https" }

	// Eight fetches of 50ms each, four at a time: two rounds, not eight.
	// Map yields in source order whatever order the calls finish in, and
	// CollectAll keeps going past the failure and reports it.
	start := time.Now()
	pages, errs := catena.FromSlice(urls).
		Filter(https).
		Then(par.Map(ctx, 4, fetch)).
		CollectAll()
	fmt.Printf("fetched %d pages in about %v\n", len(pages), time.Since(start).Round(50*time.Millisecond))
	for _, p := range pages[:3] {
		fmt.Printf("  %s (%d bytes)\n", p.URL, p.Bytes)
	}
	fmt.Println("failures:", errs)

	// The same stage under a different policy: Collect stops at the
	// first error. The calls still in flight are cancelled through their
	// context and joined before Collect returns.
	var started atomic.Int32
	counted := func(ctx context.Context, url string) (Page, error) {
		started.Add(1)
		return fetch(ctx, url)
	}
	got, err := catena.FromSlice(urls).
		Filter(https).
		Then(par.Map(ctx, 2, counted)).
		Collect()
	fmt.Printf("stop at first error: %d pages, then %v\n", len(got), err)
	fmt.Printf("  %d of 8 fetches were ever started\n", started.Load())

	// MapUnordered yields each result as its call finishes: the same
	// elements, in completion order. Sort when the order matters.
	sizes := catena.FromSlice(urls).
		Filter(https).
		Then(par.MapUnordered(ctx, 4, fetch)).
		Ignore().
		Map(func(p Page) int { return p.Bytes }).
		Collect()
	slices.Sort(sizes)
	fmt.Println("page sizes:", sizes)

	// ForEach is the terminal: run for effect, get the first error back.
	var total atomic.Int64
	err = catena.Range(1, 101, 1).Then(par.ForEach(ctx, 8, func(_ context.Context, n int) error {
		total.Add(int64(n))
		return nil
	}))
	fmt.Println("sum of 1..100, eight at a time:", total.Load(), err)

	// ChunkedWithin batches by size OR by time. This source sends a burst
	// of five, goes quiet, then sends two more: the burst closes a batch
	// on size, and the deadline flushes what the silence left behind —
	// where Chunked would hold it until more arrived.
	bursty := catena.Seq[int](func(yield func(int) bool) {
		for i := range 7 {
			if i == 5 {
				time.Sleep(300 * time.Millisecond)
			}
			if !yield(i) {
				return
			}
		}
	})
	fmt.Println("batches of up to 3, or 100ms:")
	for batch := range bursty.Then(par.ChunkedWithin[int](3, 100*time.Millisecond)) {
		fmt.Printf("  %v\n", batch)
	}
}
