// Resources: a producer that owns something — here a real temp file —
// opens it lazily inside the iteration closure. Building the pipeline
// touches nothing; consuming it opens, and any exit path closes, even
// when a downstream stage stops early. Cancellation enters the same way,
// at the edge, with UntilDone.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/NerdMeNot/catena"
)

// FileLines is the lazy-acquisition pattern from docs/error-handling.md:
// take the NAME, not an open handle. The opens/closes counters exist only
// so this example can prove the lifecycle claims it makes.
func FileLines(path string, opens, closes *int) catena.Try[string] {
	return func(yield func(string, error) bool) {
		*opens++
		f, err := os.Open(path)
		if err != nil {
			yield("", err)
			return
		}
		defer func() { *closes++; f.Close() }()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if !yield(sc.Text(), nil) {
				return // the defer still runs
			}
		}
		if err := sc.Err(); err != nil {
			yield("", err)
		}
	}
}

func main() {
	path := writeTempLog()
	defer os.Remove(path)

	opens, closes := 0, 0
	lines := FileLines(path, &opens, &closes)

	// Building a pipeline is free: nothing has been opened yet.
	warnings := lines.Ignore().
		Filter(func(l string) bool { return strings.HasPrefix(l, "WARN") }).
		Take(2)
	fmt.Printf("pipeline built: opens=%d\n", opens)

	// Consuming opens once — and Take(2) stopping early still closes,
	// because early termination returns through the producer's defer.
	fmt.Println("first two warnings:", warnings.Collect())
	fmt.Printf("after consuming:   opens=%d closes=%d\n", opens, closes)

	// UntilDone gates a pipeline on a context. When the context dies the
	// sequence yields the context's error and stops — and the producer's
	// cleanup runs exactly as above, because it is just an early exit.
	ctx, cancel := context.WithCancel(context.Background())
	seen := 0
	var got []string
	for line, err := range FileLines(path, &opens, &closes).Ignore().UntilDone(ctx).Seq2() {
		if err != nil {
			fmt.Println("stopped by:", err)
			break
		}
		got = append(got, line)
		if seen++; seen == 3 {
			cancel()
		}
	}
	fmt.Printf("read %d lines before cancellation; closes=%d\n", len(got), closes)
}

func writeTempLog() string {
	f, err := os.CreateTemp("", "catena-example-*.log")
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(f, "INFO starting up")
	fmt.Fprintln(f, "WARN disk at 81%")
	fmt.Fprintln(f, "INFO handled 42 requests")
	fmt.Fprintln(f, "WARN disk at 92%")
	fmt.Fprintln(f, "WARN disk at 97%")
	fmt.Fprintln(f, "INFO shutting down")
	f.Close()
	return f.Name()
}
