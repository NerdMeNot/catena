package catena_test

// Examples for the constructors and the slicing operators. See
// example_filtering_test.go for why these are Example functions.

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/NerdMeNot/catena"
)

// ---- creating a sequence ----

func ExampleOf() {
	// For a handful of values known at the call site: the regions to
	// probe, turned into the URLs to fetch.
	regions := catena.Of("eu-west-1", "us-east-1")

	regions.
		Map(func(r string) string { return "https://" + r + ".api.example.com/health" }).
		ForEach(func(url string) { fmt.Println(url) })
	// Output:
	// https://eu-west-1.api.example.com/health
	// https://us-east-1.api.example.com/health
}

func ExampleFromSlice() {
	// The slice is not copied, which is what makes this free — and makes
	// the pipeline a live view: finish a job and the next iteration sees
	// it.
	type job struct {
		Name string
		Done bool
	}
	jobs := []job{{"lint", true}, {"test", false}, {"deploy", false}}
	pending := catena.FromSlice(jobs).
		FilterNot(func(j job) bool { return j.Done }).
		Map(func(j job) string { return j.Name })

	fmt.Println(pending.Collect())
	jobs[1].Done = true
	fmt.Println(pending.Collect())
	// Output:
	// [test deploy]
	// [deploy]
}

func ExampleFrom() {
	// Takes the literal function type, so any iterator adapts without a
	// conversion at the call site — including the standard library's.
	path := "/usr/local/bin:/usr/bin:/bin:/opt/homebrew/bin"

	fmt.Println(catena.From(strings.SplitSeq(path, ":")).
		Filter(func(dir string) bool { return strings.HasPrefix(dir, "/usr/") }).
		Collect())
	// Output: [/usr/local/bin /usr/bin]
}

func ExampleFromMap() {
	// Which products are sold out? A map iterates in no particular
	// order, so sort before anything order-sensitive such as printing.
	stock := map[string]int{"keyboard": 12, "monitor": 0, "webcam": 0, "mouse": 40}
	soldOut := catena.FromMap(stock).
		Filter(func(_ string, qty int) bool { return qty == 0 }).
		Keys()

	fmt.Println(catena.Sorted(soldOut).Collect())
	// Output: [monitor webcam]
}

func ExampleFrom2() {
	// Any func(yield func(K, V) bool) adapts — here the standard
	// library's index/value iterator, for progress labels.
	steps := []string{"checkout", "build", "test"}

	catena.From2(slices.All(steps)).
		MapTo(func(i int, step string) string {
			return fmt.Sprintf("[%d/%d] %s", i+1, len(steps), step)
		}).
		ForEach(func(label string) { fmt.Println(label) })
	// Output:
	// [1/3] checkout
	// [2/3] build
	// [3/3] test
}

func ExampleFromErrs() {
	// Wraps a source that can fail part-way — a parser, a database
	// cursor, a network stream — so the error travels with the values
	// instead of being checked at every step.
	fields := []string{"21.5", "22.1", "n/a", "23.0"}
	readings := catena.FromErrs(func(yield func(float64, error) bool) {
		for _, f := range fields {
			if !yield(strconv.ParseFloat(f, 64)) {
				return
			}
		}
	})

	vals, err := readings.Collect()
	fmt.Println(vals, err)
	// Output: [21.5 22.1] strconv.ParseFloat: parsing "n/a": invalid syntax
}

func ExampleFromChan() {
	results := make(chan string, 3)
	for _, name := range []string{"a.png", "b.png", "c.png"} {
		results <- "resized " + name
	}
	close(results)

	// Drains a worker pool's results until the channel closes or ctx is
	// done. Single-use, and it starts no goroutine: a sequence that is
	// never consumed never receives.
	fmt.Println(catena.FromChan(context.Background(), results).Collect())
	// Output: [resized a.png resized b.png resized c.png]
}

func ExampleEmpty() {
	// The "nothing to report" return value: callers keep chaining on it
	// without a nil check.
	ordersFor := func(customer string) catena.Seq[string] {
		if customer == "" {
			return catena.Empty[string]()
		}
		return catena.Of("A-1001", "A-1002")
	}

	fmt.Println(ordersFor("").Count(), ordersFor("ada").Count())
	// Output: 0 2
}

func ExampleEmpty2() {
	// The pair-sequence counterpart: a lookup with no matches still
	// collects to a usable, empty map.
	labels := catena.Empty2[string, string]()

	fmt.Println(labels.Count(), catena.CollectMap(labels))
	// Output: 0 map[]
}

func ExampleEmptyTry() {
	// A fallible source with nothing in it, such as a stubbed query in a
	// test: no rows, and no error either.
	rows, err := catena.EmptyTry[string]().Collect()

	fmt.Println(len(rows), err)
	// Output: 0 <nil>
}

func ExampleOnce1() {
	// Lifts the one value you have into a sequence — here a header row
	// to put ahead of the data. Once1, not Once: Once is the single-use
	// guard method on Seq.
	rows := catena.Of("ada,36", "bob,41")

	fmt.Println(catena.Once1("name,age").Concat(rows).Collect())
	// Output: [name,age ada,36 bob,41]
}

func ExampleRepeat() {
	// Infinite, so it must be bounded by something downstream — here
	// Zip, which stops when the hosts run out, leaving every host with
	// the same starting status.
	hosts := catena.Of("web-1", "web-2", "db-1")

	fmt.Println(catena.CollectMap(hosts.Zip(catena.Repeat("pending"))))
	// Output: map[db-1:pending web-1:pending web-2:pending]
}

func ExampleRepeatN() {
	// A fixed retry schedule: three attempts, the same wait before each.
	fmt.Println(catena.RepeatN(500*time.Millisecond, 3).Collect())
	// Output: [500ms 500ms 500ms]
}

func ExampleGenerate() {
	// Exponential backoff. The seed is yielded first, then next applied
	// repeatedly — forever, so Take decides how many retries there are.
	backoff := catena.Generate(100*time.Millisecond, func(d time.Duration) time.Duration {
		return d * 2
	})

	fmt.Println(backoff.Take(5).Collect())
	// Output: [100ms 200ms 400ms 800ms 1.6s]
}

func ExampleGenerateWhile() {
	// Follows an API's "next page" links until there are none. The seed
	// is yielded unconditionally; a value produced alongside ok=false —
	// the empty link after the last page — is not.
	nextLink := map[string]string{
		"/orders?page=1": "/orders?page=2",
		"/orders?page=2": "/orders?page=3",
	}
	pages := catena.GenerateWhile("/orders?page=1", func(url string) (string, bool) {
		next, ok := nextLink[url]
		return next, ok
	})

	fmt.Println(pages.Collect())
	// Output: [/orders?page=1 /orders?page=2 /orders?page=3]
}

func ExampleRange() {
	// Half-open, like a slice expression: the offsets of each page of 90
	// results, then a countdown. A sign mismatch between step and
	// direction yields nothing rather than panicking, so a computed step
	// is safe.
	total, pageSize := 90, 25

	fmt.Println(catena.Range(0, total, pageSize).Collect())
	fmt.Println(catena.Range(3, 0, -1).Collect())
	fmt.Println(catena.Range(0, total, -pageSize).Collect())
	// Output:
	// [0 25 50 75]
	// [3 2 1]
	// []
}

func ExampleCycle() {
	// Round-robin: Zip stops when the tickets run out, so the endless
	// rota is never a problem. Over an empty source Cycle terminates
	// rather than spinning.
	tickets := catena.Of("T-101", "T-102", "T-103", "T-104", "T-105")
	rota := catena.Cycle(catena.Of("ada", "bob"))

	fmt.Println(tickets.Zip(rota).
		MapTo(func(ticket, engineer string) string { return ticket + ":" + engineer }).
		Collect())
	fmt.Println(catena.Cycle(catena.Empty[string]()).Collect())
	// Output:
	// [T-101:ada T-102:bob T-103:ada T-104:bob T-105:ada]
	// []
}

func ExampleSelf() {
	// The identity selector, for the -By operators when the element is
	// already the key: counting responses by status code.
	statuses := catena.Of(200, 404, 200, 500, 200, 404)

	fmt.Println(statuses.TallyBy(catena.Self[int]))
	// Output: map[200:3 404:2 500:1]
}

// ---- slicing ----

func ExampleSeq_Take() {
	// The next three invoice numbers from a counter that never ends.
	// Take consumes exactly what it emits, so it bounds an infinite
	// source.
	invoices := catena.Generate(1041, func(n int) int { return n + 1 }).
		Map(func(n int) string { return fmt.Sprintf("INV-%d", n) })

	fmt.Println(invoices.Take(3).Collect())
	// Output: [INV-1041 INV-1042 INV-1043]
}

func ExampleSeq_TakeWhile() {
	// The build steps that passed before the first failure. It stops at
	// the first element that fails — unlike Filter, which would keep
	// testing the rest and let "package" through.
	type step struct {
		Name   string
		Passed bool
	}
	steps := catena.Of(
		step{"lint", true}, step{"build", true},
		step{"test", false}, step{"package", true},
	)

	fmt.Println(steps.
		TakeWhile(func(s step) bool { return s.Passed }).
		Map(func(s step) string { return s.Name }).
		Collect())
	// Output: [lint build]
}

func ExampleSeq_TakeLast() {
	// tail -n 2: the end of a log. It buffers only n lines, but has to
	// read the source to its end to know which they are.
	log := catena.Of(
		"12:00:01 starting",
		"12:00:02 listening on :8080",
		"12:04:37 upstream timeout",
		"12:04:38 shutting down",
	)

	log.TakeLast(2).ForEach(func(line string) { fmt.Println(line) })
	// Output:
	// 12:04:37 upstream timeout
	// 12:04:38 shutting down
}

func ExampleSeq_Drop() {
	// Pagination: skip the pages already shown, then take one page.
	results := catena.Of("ada", "bob", "cyd", "dee", "eve", "fay", "gus")
	page, size := 1, 3

	fmt.Println(results.Drop(page * size).Take(size).Collect())
	// Output: [dee eve fay]
}

func ExampleSeq_DropWhile() {
	// Skips the comment banner at the top of a config file. Only the
	// leading run is dropped; once the predicate fails, everything after
	// is kept — including the later comment.
	config := catena.Of(
		"# managed by ops",
		"# do not edit",
		"port = 8080",
		"# seconds",
		"timeout = 30",
	)

	fmt.Println(config.
		DropWhile(func(line string) bool { return strings.HasPrefix(line, "#") }).
		JoinToString("; ", catena.Self[string]))
	// Output: port = 8080; # seconds; timeout = 30
}

func ExampleSeq_DropLast() {
	// Strips a totals row off the end of an export without knowing how
	// long the export is. It runs n elements behind the source, and
	// buffers only those.
	export := catena.Of("ada,36.00", "bob,41.50", "eve,12.25", "TOTAL,89.75")

	fmt.Println(export.DropLast(1).Collect())
	// Output: [ada,36.00 bob,41.50 eve,12.25]
}

func ExampleSeq_Step() {
	// Downsampling: one CPU reading in every three is enough for the
	// chart. The first element always survives, then every nth after it.
	cpuPercent := catena.Of(12, 15, 11, 40, 42, 38, 90, 88)

	fmt.Println(cpuPercent.Step(3).Collect())
	// Output: [12 40 90]
}
