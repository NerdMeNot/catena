package catena_test

// Examples for transforming, combining, ordering and batching. See
// example_filtering_test.go for why these are Example functions.

import (
	"cmp"
	"fmt"
	"strings"
	"time"

	"github.com/NerdMeNot/catena"
)

// ---- transforming ----

func ExampleSeq_Map() {
	// The element type changes mid-chain — accounts in, addresses out —
	// which is what generic methods made possible.
	type account struct {
		Name  string
		Email string
	}
	accounts := catena.Of(
		account{"Ada", "ada@example.com"},
		account{"Bob", "bob@example.com"},
	)
	fmt.Println(accounts.Map(func(a account) string { return a.Email }).Collect())
	// Output: [ada@example.com bob@example.com]
}

func ExampleSeq_MapIndexed() {
	// The position arrives with the element, so numbering needs no counter
	// kept outside the chain. It counts from 0; a ranking wants i+1.
	finishers := catena.Of("kiptum", "bekele", "farah")
	fmt.Println(finishers.
		MapIndexed(func(i int, name string) string { return fmt.Sprintf("%d. %s", i+1, name) }).
		Collect())
	// Output: [1. kiptum 2. bekele 3. farah]
}

func ExampleSeq_FilterMap() {
	// Fused filter and map: keep the error lines and strip their prefix in
	// one stage. Anything returning (value, bool) fits, and the test is
	// not run a second time to get the value.
	logs := catena.Of(
		"INFO listening on :8080",
		"ERROR db: connection refused",
		"INFO GET /health 200",
		"ERROR cache: timeout",
	)
	for msg := range logs.FilterMap(func(line string) (string, bool) {
		return strings.CutPrefix(line, "ERROR ")
	}).Seq() {
		fmt.Println(msg)
	}
	// Output:
	// db: connection refused
	// cache: timeout
}

func ExampleSeq_FlatMap() {
	// Each element expands into a sequence of its own and the results run
	// together in order. The inner sequences stay lazy: no port list is
	// built for a rule until the consumer reaches it.
	type rule struct {
		Service  string
		From, To int
	}
	rules := catena.Of(rule{"web", 8080, 8082}, rule{"metrics", 9100, 9101})
	fmt.Println(rules.
		FlatMap(func(r rule) catena.Seq[int] { return catena.Range(r.From, r.To+1, 1) }).
		Collect())
	// Output: [8080 8081 8082 9100 9101]
}

func ExampleSeq_FlatMapSlice() {
	// The same, when each element already holds a slice: every file touched
	// by a run of commits, with no wrapping of the slice in a sequence.
	type commit struct {
		Hash  string
		Files []string
	}
	commits := catena.Of(
		commit{"a1f9", []string{"go.mod", "main.go"}},
		commit{"c03d", []string{"README.md"}},
	)
	fmt.Println(commits.FlatMapSlice(func(c commit) []string { return c.Files }).Collect())
	// Output: [go.mod main.go README.md]
}

func ExampleSeq_Scan() {
	// A running fold: the balance after every transaction rather than only
	// the final one. The opening balance itself is not emitted, so the
	// output is as long as the input.
	opening := 500
	transactions := catena.Of(120, -75, -300, 40)
	fmt.Println(transactions.
		Scan(opening, func(balance, amount int) int { return balance + amount }).
		Collect())
	// Output: [620 545 245 285]
}

func ExampleSeq_MapErr() {
	// A mapping that can fail produces a Try, so one bad timeout in a
	// config does not have to abort the rest. A failed call yields the zero
	// value alongside its error, never a half-built one.
	timeouts := catena.Of("30s", "soon", "5m")
	parsed, errs := timeouts.MapErr(time.ParseDuration).CollectAll()
	fmt.Println(parsed, len(errs))
	// Output: [30s 5m0s] 1
}

func ExampleSeq_OnEach() {
	// Side effects without changing the stream — logging, metrics,
	// progress. Elements pass through untouched, and nothing is logged
	// until the terminal operation pulls them.
	type upload struct {
		Name  string
		Bytes int
	}
	uploads := catena.Of(upload{"report.pdf", 48_200}, upload{"logo.png", 9_100})
	total := uploads.
		OnEach(func(u upload) { fmt.Println("uploading", u.Name) }).
		SumOf(func(u upload) int { return u.Bytes })
	fmt.Println(total, "bytes")
	// Output:
	// uploading report.pdf
	// uploading logo.png
	// 57300 bytes
}

func ExampleFlatten() {
	// One stream from a sequence of sequences — here, the hosts of each
	// region — in order, and still lazy.
	byRegion := catena.Of(
		catena.Of("eu-1.example.com", "eu-2.example.com"),
		catena.Of("us-1.example.com"),
	)
	fmt.Println(catena.Flatten(byRegion).Collect())
	// Output: [eu-1.example.com eu-2.example.com us-1.example.com]
}

func ExampleFlattenSlices() {
	// The same for slices: a paginated API hands back a page at a time,
	// and the caller wants the rows.
	pages := catena.Of(
		[]string{"invoice-101", "invoice-102"},
		[]string{"invoice-103"},
	)
	fmt.Println(catena.FlattenSlices(pages).Collect())
	// Output: [invoice-101 invoice-102 invoice-103]
}

// ---- combining ----

func ExampleSeq_Concat() {
	// One sequence after another, any number of them: resolvers in the
	// order they should be tried. No sequence is read until the one before
	// it is exhausted.
	primary := catena.Of("10.0.0.2", "10.0.0.3")
	office := catena.Of("192.168.1.1")
	public := catena.Of("1.1.1.1", "8.8.8.8")
	fmt.Println(primary.Concat(office, public).Collect())
	// Output: [10.0.0.2 10.0.0.3 192.168.1.1 1.1.1.1 8.8.8.8]
}

func ExampleSeq_Append() {
	// Loose values on the end, without wrapping them in a sequence first:
	// the flags a wrapper always adds after the user's own arguments.
	userArgs := catena.Of("test", "./...")
	fmt.Println(userArgs.Append("-race", "-count=1").Collect())
	// Output: [test ./... -race -count=1]
}

func ExampleSeq_Prepend() {
	// Loose values on the front: a header row ahead of the data, without
	// building a second sequence just to hold it.
	rows := catena.Of("report.pdf,48200", "logo.png,9100")
	for line := range rows.Prepend("name,bytes").Seq() {
		fmt.Println(line)
	}
	// Output:
	// name,bytes
	// report.pdf,48200
	// logo.png,9100
}

func ExampleChain() {
	// The variadic form of Concat, for when the sequences are in a slice
	// already and none of them is the obvious receiver — one per rotated
	// log file, say.
	rotated := []catena.Seq[string]{
		catena.Of("mon:start", "mon:stop"),
		catena.Of("tue:start"),
		catena.Of("wed:start", "wed:crash"),
	}
	fmt.Println(catena.Chain(rotated...).Collect())
	// Output: [mon:start mon:stop tue:start wed:start wed:crash]
}

func ExampleSeq_Zip() {
	// Pairs elements positionally and stops at the shorter side: a CSV row
	// that is missing its last field yields only the columns it has.
	header := catena.Of("host", "port", "region")
	row := catena.Of("db-1", "5432")
	fmt.Println(header.Zip(row).
		MapTo(func(column, value string) string { return column + "=" + value }).
		Collect())
	// Output: [host=db-1 port=5432]
}

func ExampleSeq_ZipWithNext() {
	// Each element paired with its successor — deltas, gaps, transitions.
	// Here, the seconds between heartbeats; the long gap is the outage.
	// Four heartbeats make three gaps.
	heartbeats := catena.Of(0, 30, 61, 245)
	fmt.Println(heartbeats.ZipWithNext().
		MapTo(func(prev, next int) int { return next - prev }).
		Collect())
	// Output: [30 31 184]
}

func ExampleSeq_WithIndex() {
	// The index becomes half of a pair, so it survives later stages: the
	// positions reported are those in the original file, counting from 0,
	// not those in the filtered result.
	lines := catena.Of("package main", "// TODO: add flags", "func main() {", "// TODO: exit code", "}")
	fmt.Println(lines.WithIndex().
		Filter(func(_ int, line string) bool { return strings.Contains(line, "TODO") }).
		Keys().
		Collect())
	// Output: [1 3]
}

func ExampleSeq_JoinBy() {
	// A relational inner join. Rows without a match are dropped on both
	// sides — the order for unknown customer 9, and eve, who ordered
	// nothing — and a key that matches twice produces a row each time.
	type order struct {
		Customer int
		Amount   int
	}
	type customer struct {
		ID   int
		Name string
	}
	orders := catena.Of(order{1, 30}, order{2, 10}, order{9, 99}, order{1, 45})
	customers := catena.Of(customer{1, "ada"}, customer{2, "bob"}, customer{3, "eve"})

	fmt.Println(orders.JoinBy(customers,
		func(o order) int { return o.Customer },
		func(c customer) int { return c.ID },
		func(o order, c customer) string { return fmt.Sprintf("%s:%d", c.Name, o.Amount) },
	).Collect())
	// Output: [ada:30 bob:10 ada:45]
}

func ExampleSeq_LeftJoinBy() {
	// A left outer join: the order with no customer on file is kept, and
	// combine is told so, receiving a zero customer and false.
	type order struct {
		Customer int
		Amount   int
	}
	type customer struct {
		ID   int
		Name string
	}
	orders := catena.Of(order{1, 30}, order{9, 99})
	customers := catena.Of(customer{1, "ada"})

	fmt.Println(orders.LeftJoinBy(customers,
		func(o order) int { return o.Customer },
		func(c customer) int { return c.ID },
		func(o order, c customer, ok bool) string {
			if !ok {
				return fmt.Sprintf("guest:%d", o.Amount)
			}
			return fmt.Sprintf("%s:%d", c.Name, o.Amount)
		},
	).Collect())
	// Output: [ada:30 guest:99]
}

func ExampleSeq_GroupJoinBy() {
	// One row per customer with all of their orders together — where
	// JoinBy would repeat ada once per order and drop bob, who has none.
	type order struct {
		Customer int
		Amount   int
	}
	type customer struct {
		ID   int
		Name string
	}
	customers := catena.Of(customer{1, "ada"}, customer{2, "bob"})
	orders := catena.Of(order{1, 30}, order{1, 12}, order{9, 99})

	fmt.Println(customers.GroupJoinBy(orders,
		func(c customer) int { return c.ID },
		func(o order) int { return o.Customer },
		func(c customer, os []order) string { return fmt.Sprintf("%s:%d", c.Name, len(os)) },
	).Collect())
	// Output: [ada:2 bob:0]
}

func ExampleSeq_Then() {
	// Package functions sit inside the chain instead of wrapping it, so
	// the steps read in the order they happen: dedupe the hosts, sort
	// them, roll out two at a time. A function taking more than the
	// sequence goes in a closure.
	hosts := catena.Of("web-2", "web-1", "web-2", "web-3", "web-1")
	fmt.Println(hosts.
		Then(catena.Distinct).
		Then(catena.Sorted).
		Then(func(s catena.Seq[string]) catena.Seq[[]string] { return catena.Chunked(s, 2) }).
		Collect())
	// Output: [[web-1 web-2] [web-3]]
}

func ExampleDuplicates() {
	// Each value that occurs more than once, reported once however often
	// it repeats — in the order the repeats were found.
	requestIDs := catena.Of("r-71", "r-30", "r-92", "r-30", "r-71", "r-71")
	fmt.Println(catena.Duplicates(requestIDs).Collect())
	// Output: [r-30 r-71]
}

func ExampleSeq_DuplicatesBy() {
	// The element reported is the one that repeats the key: here, the
	// second signup with each address, which is the one to reject.
	type signup struct {
		Email string
		Name  string
	}
	signups := catena.Of(
		signup{"ada@example.com", "Ada"},
		signup{"bob@example.com", "Bob"},
		signup{"ada@example.com", "Ada L."},
	)
	fmt.Println(signups.DuplicatesBy(func(s signup) string { return s.Email }).Collect())
	// Output: [{ada@example.com Ada L.}]
}

func ExampleIsSorted() {
	// One pass, no buffer, and it stops at the first pair out of order.
	// Equal neighbours count as sorted: two events in the same second are
	// fine, a timestamp that goes backwards is not.
	steady := catena.Of(1700000001, 1700000004, 1700000004, 1700000009)
	skewed := catena.Of(1700000001, 1700000009, 1700000004)
	fmt.Println(catena.IsSorted(steady), catena.IsSorted(skewed))
	// Output: true false
}

func ExampleSeq_IsSortedBy() {
	// Checks an ordering by one field without sorting anything: is the
	// changelog oldest-first? ISO dates compare correctly as strings.
	type release struct {
		Tag  string
		Date string
	}
	changelog := catena.Of(
		release{"v1.0.0", "2025-11-03"},
		release{"v1.1.0", "2026-01-19"},
		release{"v1.0.1", "2026-01-12"},
	)
	fmt.Println(changelog.IsSortedBy(func(r release) string { return r.Date }))
	// Output: false
}

func ExampleSeq_IsSortedWith() {
	// Any ordering a comparator can express: here, highest score first.
	// A tie is still in order.
	type player struct {
		Name  string
		Score int
	}
	board := catena.Of(player{"ada", 90}, player{"bob", 85}, player{"eve", 85}, player{"dan", 70})
	fmt.Println(board.IsSortedWith(func(a, b player) int { return cmp.Compare(b.Score, a.Score) }))
	// Output: true
}

func ExampleUnion() {
	// Set semantics: a permission granted twice, or by both the role and
	// the team, appears once. Encounter order, left operand first.
	fromRole := catena.Of("read", "write", "write")
	fromTeam := catena.Of("deploy", "read")
	fmt.Println(catena.Union(fromRole, fromTeam).Collect())
	// Output: [read write deploy]
}

func ExampleIntersect() {
	// What both sides have — the users active last week who came back
	// this week — in the order of the first operand.
	lastWeek := catena.Of("ada", "bob", "eve", "dan")
	thisWeek := catena.Of("eve", "zoe", "ada")
	fmt.Println(catena.Intersect(lastWeek, thisWeek).Collect())
	// Output: [ada eve]
}

func ExampleExcept() {
	// What the first side has that the second does not: the hosts still
	// waiting for a patch.
	fleet := catena.Of("web-1", "web-2", "db-1", "cache-1")
	patched := catena.Of("web-2", "cache-1")
	fmt.Println(catena.Except(fleet, patched).Collect())
	// Output: [web-1 db-1]
}

func ExampleSeq_Intersperse() {
	// A separator between elements but not before the first or after the
	// last — the fence-post problem, handled once.
	menu := catena.Of("Open", "Save", "Quit")
	for line := range menu.Intersperse("----").Seq() {
		fmt.Println(line)
	}
	// Output:
	// Open
	// ----
	// Save
	// ----
	// Quit
}

func ExampleSeq_IfEmpty() {
	// A fallback for the whole sequence, not per element: the default
	// applies only when nothing at all was configured.
	configured := catena.Of("eu-1.example.com", "eu-2.example.com")
	unset := catena.Empty[string]()
	fmt.Println(configured.IfEmpty("localhost").Collect())
	fmt.Println(unset.IfEmpty("localhost").Collect())
	// Output:
	// [eu-1.example.com eu-2.example.com]
	// [localhost]
}

// ---- ordering ----

func ExampleSorted() {
	// Ascending order for any ordered type. A map hands its keys back in a
	// different order on every run; sorting them makes the output stable.
	env := map[string]string{"PORT": "8080", "HOME": "/srv/app", "LOG_LEVEL": "info"}
	fmt.Println(catena.Sorted(catena.FromMap(env).Keys()).Collect())
	// Output: [HOME LOG_LEVEL PORT]
}

func ExampleSortedDesc() {
	// Largest first: the slowest responses, in milliseconds, at the top
	// of the report.
	latencies := catena.Of(87, 912, 45, 340)
	fmt.Println(catena.SortedDesc(latencies).Collect())
	// Output: [912 340 87 45]
}

func ExampleSeq_SortedBy() {
	// Stable, and the selector runs exactly once per element rather than
	// once per comparison — so an expensive key is affordable. Stability
	// shows here: tickets of equal priority stay in the order they arrived.
	type ticket struct {
		ID       string
		Priority int
	}
	queue := catena.Of(ticket{"T-1", 2}, ticket{"T-2", 1}, ticket{"T-3", 2}, ticket{"T-4", 1})
	fmt.Println(queue.SortedBy(func(t ticket) int { return t.Priority }).Collect())
	// Output: [{T-2 1} {T-4 1} {T-1 2} {T-3 2}]
}

func ExampleSeq_SortedByDesc() {
	// Largest key first: what is filling the disk.
	type file struct {
		Name  string
		Bytes int
	}
	files := catena.Of(file{"app.log", 9_400}, file{"core.dump", 880_000}, file{"cache.db", 61_000})
	fmt.Println(files.SortedByDesc(func(f file) int { return f.Bytes }).Collect())
	// Output: [{core.dump 880000} {cache.db 61000} {app.log 9400}]
}

func ExampleSeq_SortedWith() {
	// A comparator, for orderings a single key cannot express: by region,
	// then busiest first within each region.
	type host struct {
		Region string
		Name   string
		Load   int
	}
	hosts := catena.Of(host{"us", "us-1", 40}, host{"eu", "eu-1", 15}, host{"us", "us-2", 85}, host{"eu", "eu-2", 60})
	fmt.Println(hosts.
		SortedWith(func(a, b host) int {
			return cmp.Or(strings.Compare(a.Region, b.Region), cmp.Compare(b.Load, a.Load))
		}).
		Map(func(h host) string { return h.Name }).
		Collect())
	// Output: [eu-2 eu-1 us-2 us-1]
}

func ExampleSeq_Reversed() {
	// Last in, first out: migrations are rolled back in the opposite
	// order to the one they were applied in.
	applied := catena.Of("001_users", "002_orders", "003_add_index")
	fmt.Println(applied.Reversed().Collect())
	// Output: [003_add_index 002_orders 001_users]
}

// ---- batching ----

func ExampleChunked() {
	// Fixed-size batches, for an API that takes so many IDs per request;
	// the last one may be short. Every chunk is a fresh slice, so keeping
	// one is safe.
	userIDs := catena.Range(101, 108, 1)
	for batch := range catena.Chunked(userIDs, 3).Seq() {
		fmt.Println("DELETE /users", batch)
	}
	// Output:
	// DELETE /users [101 102 103]
	// DELETE /users [104 105 106]
	// DELETE /users [107]
}

func ExampleChunkedBy() {
	// A chunk per run of equal keys — the streaming answer to grouping
	// input that is already ordered, in memory bounded by the longest run.
	// It splits on change, not on value: a host that reappears later
	// starts a new chunk.
	type entry struct {
		Host string
		Path string
	}
	access := catena.Of(
		entry{"web-1", "/"}, entry{"web-1", "/cart"},
		entry{"web-2", "/"},
		entry{"web-1", "/pay"},
	)
	for run := range catena.ChunkedBy(access, func(e entry) string { return e.Host }).Seq() {
		fmt.Println(run[0].Host, len(run))
	}
	// Output:
	// web-1 2
	// web-2 1
	// web-1 1
}

func ExampleWindowed() {
	// Overlapping windows: size 3 advancing by 1, which is a moving
	// average — the spike is smoothed in rather than jumping. Only full
	// windows are yielded, so five samples make three.
	latencies := catena.Of(120, 130, 110, 400, 420)
	for w := range catena.Windowed(latencies, 3, 1).Seq() {
		fmt.Println(w, "avg", catena.Sum(catena.FromSlice(w))/len(w))
	}
	// Output:
	// [120 130 110] avg 120
	// [130 110 400] avg 213
	// [110 400 420] avg 310
}
