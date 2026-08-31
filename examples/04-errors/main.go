// Errors: one parse pipeline, three policies. Try's design is that the
// pipeline carries errors and the CONSUMER decides what they mean —
// abort, gather, or skip — so the same stages serve all three.
package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/NerdMeNot/catena"
)

// A CSV-ish feed where line 3 is corrupt and line 5 has a bad quantity.
var feed = []string{
	"widget,4",
	"gadget,11",
	"THIS LINE IS GARBAGE",
	"sprocket,2",
	"doohickey,many",
	"gizmo,7",
}

type Item struct {
	Name string
	Qty  int
}

func parse(line string) (Item, error) {
	name, qty, ok := strings.Cut(line, ",")
	if !ok {
		return Item{}, fmt.Errorf("no comma in %q", line)
	}
	n, err := strconv.Atoi(qty)
	if err != nil {
		return Item{}, fmt.Errorf("bad quantity for %s: %w", name, err)
	}
	return Item{name, n}, nil
}

type numbered struct {
	n    int
	line string
}

func main() {
	// The pipeline, defined once. WithIndex carries the line number
	// alongside each line, so errors can say where they came from — the
	// stage that knows the position is the stage that reports it. Every
	// stage is stateless, so this one value is safely consumed by all
	// three policies below.
	items := catena.FromSlice(feed).
		WithIndex().
		MapTo(func(i int, line string) numbered { return numbered{i + 1, line} }).
		MapErr(func(x numbered) (Item, error) {
			it, err := parse(x.line)
			if err != nil {
				return Item{}, fmt.Errorf("line %d: %w", x.n, err)
			}
			return it, nil
		})

	// Policy 1 — abort: partial results up to the first error, plus it.
	// WrapErr adds another layer of context on the way out.
	vals, err := items.
		WrapErr(func(err error) error { return fmt.Errorf("inventory feed: %w", err) }).
		Collect()
	fmt.Printf("abort:  parsed %d items, then: %v\n", len(vals), err)

	// Policy 2 — gather: everything good AND everything bad, one pass.
	good, errs := items.CollectAll()
	fmt.Printf("gather: %d good, %d bad\n", len(good), len(errs))
	for _, e := range errs {
		fmt.Println("        -", e)
	}

	// Policy 3 — skip: drop failures and keep going. After Ignore() it
	// is a plain Seq again, with the whole API available.
	total := items.Ignore().SumOf(func(it Item) int { return it.Qty })
	fmt.Println("skip:   total quantity across parseable lines:", total)

	// Recover: errors can be repaired mid-stream when a fallback exists.
	// Here an unparseable quantity becomes zero rather than a failure;
	// the garbage line stays an error.
	patched, stillBad := items.
		Recover(func(err error) (Item, bool) {
			if strings.Contains(err.Error(), "bad quantity") {
				return Item{Name: "(defaulted)", Qty: 0}, true
			}
			return Item{}, false
		}).
		CollectAll()
	fmt.Printf("repair: %d items, %d unrepairable\n", len(patched), len(stillBad))
}
