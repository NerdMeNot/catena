package catena_test

// Examples for consuming, searching, folding and aggregating. See
// example_filtering_test.go for why these are Example functions.

import (
	"cmp"
	"context"
	"fmt"
	"math/rand/v2"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/NerdMeNot/catena"
)

// ---- consuming ----

func ExampleSeq_Collect() {
	// The hand-off back to ordinary Go: a plain slice. Nothing matching
	// gives nil, as slices.Collect does, so len and range need no guard.
	addrs := catena.Of("web-1:443", "db-1:5432", "cache-1:6379")
	onPort := func(port string) func(string) bool {
		return func(addr string) bool { return strings.HasSuffix(addr, port) }
	}
	tls := addrs.Filter(onPort(":443")).Collect()
	plain := addrs.Filter(onPort(":80")).Collect()
	fmt.Println(tls, plain == nil)
	// Output: [web-1:443] true
}

func ExampleSeq_ToList() {
	// The eager twin: a List has the same operations, evaluated at once,
	// so it can be measured, indexed and reused without re-running the
	// pipeline.
	releases := catena.Of("v1.4.0", "v1.4.1", "v1.5.0").ToList()
	fmt.Println(releases.Len(), releases.At(releases.Len()-1))
	// Output: 3 v1.5.0
}

func ExampleSeq_Seq() {
	// A free conversion to the standard iterator type, in both
	// directions — Seq IS iter.Seq underneath — so anything in the
	// standard library that takes an iterator takes a pipeline.
	hosts := catena.Of("web-2", "db-1", "web-1")
	fmt.Println(slices.Sorted(hosts.Seq()))
	// Output: [db-1 web-1 web-2]
}

func ExampleSeq_Pull() {
	// Inverts control for hand-written loops, such as reading a header
	// row before the records. THE CALLER MUST CALL stop, or a producer
	// holding a resource never releases it.
	rows := catena.Of("id,email", "1,ada@example.com", "2,bob@example.com")
	next, stop := rows.Pull()
	defer stop()

	header, _ := next()
	record, ok := next()
	fmt.Println(header, "|", record, ok)
	// Output: id,email | 1,ada@example.com true
}

func ExampleSeq_ToChan() {
	// The fan-out mechanism: a Seq is not safe to consume from two
	// goroutines, a channel is. Cancelling ctx closes the channel.
	uploads := catena.Of("a.png", "b.png", "c.png", "d.png")
	jobs := uploads.ToChan(context.Background())

	var resized atomic.Int32
	var workers sync.WaitGroup
	for range 2 {
		workers.Go(func() {
			for range jobs {
				resized.Add(1)
			}
		})
	}
	workers.Wait()
	fmt.Println(resized.Load())
	// Output: 4
}

func ExampleSeq_ForEach() {
	// The end of a pipeline whose point is the side effect.
	results := catena.Of("ok pkg/api", "FAIL pkg/db", "ok pkg/web", "FAIL pkg/auth")
	results.
		Filter(func(line string) bool { return strings.HasPrefix(line, "FAIL") }).
		ForEach(func(line string) { fmt.Println(line) })
	// Output:
	// FAIL pkg/db
	// FAIL pkg/auth
}

func ExampleSeq_ForEachIndexed() {
	// The position comes with the element, counted from zero.
	steps := catena.Of("back up the database", "run migrations", "restart the API")
	steps.ForEachIndexed(func(i int, step string) {
		fmt.Printf("%d. %s\n", i+1, step)
	})
	// Output:
	// 1. back up the database
	// 2. run migrations
	// 3. restart the API
}

func ExampleSeq_ForEachErr() {
	// Stops at the first error the callback returns, and returns it:
	// the order after the failure is never attempted.
	type order struct{ ID, Card string }
	orders := catena.Of(order{"A-1", "visa"}, order{"A-2", ""}, order{"A-3", "amex"})

	err := orders.ForEachErr(func(o order) error {
		if o.Card == "" {
			return fmt.Errorf("order %s: no card on file", o.ID)
		}
		fmt.Println("charged", o.ID)
		return nil
	})
	fmt.Println("err:", err)
	// Output:
	// charged A-1
	// err: order A-2: no card on file
}

func ExampleSeq_Drain() {
	// Runs a pipeline whose stages do the work, discarding the elements:
	// here, warming a cache.
	type user struct {
		ID   int
		Name string
	}
	cache := map[int]string{}
	catena.Of(user{7, "ada"}, user{9, "bob"}).
		OnEach(func(u user) { cache[u.ID] = u.Name }).
		Drain()
	fmt.Println(len(cache), cache[9])
	// Output: 2 bob
}

func ExampleSeq_Once() {
	// A development guard for sources that can only be read once — a
	// request body, a database cursor: the second consumption panics
	// instead of silently re-running the producer or reading nothing.
	body := catena.Of("id,total", "1,40", "2,15").Once()
	fmt.Println(body.Count(), "lines")

	defer func() { fmt.Println("recovered:", recover()) }()
	body.Collect()
	// Output:
	// 3 lines
	// recovered: catena: Once: sequence consumed more than once
}

func ExampleSeq_UntilDone() {
	// Cancellation enters at the edge rather than threading a context
	// through every stage: an endless polling loop ends when the context
	// does, and the context's error arrives as the pipeline's error.
	ctx, cancel := context.WithCancel(context.Background())
	polls := catena.Generate(1, func(attempt int) int { return attempt + 1 })

	done, err := polls.UntilDone(ctx).
		OnEach(func(attempt int) {
			if attempt == 3 { // the deadline passes, or the user hits Ctrl-C
				cancel()
			}
		}).
		Collect()
	fmt.Println(done, err)
	// Output: [1 2 3] context canceled
}

// ---- searching and testing ----

func ExampleSeq_First() {
	// The bool tells an empty sequence apart from a first element that
	// happens to be the zero value.
	queue := catena.Of("deploy api", "rotate keys")
	job, ok := queue.First()
	_, idle := catena.Empty[string]().First()
	fmt.Println(job, ok, idle)
	// Output: deploy api true false
}

func ExampleSeq_Last() {
	// The most recent of a series of health checks.
	checks := catena.Of(200, 200, 503)
	latest, ok := checks.Last()
	fmt.Println(latest, ok)
	// Output: 503 true
}

func ExampleSeq_Single() {
	// True only for exactly one element — the check for "there must be
	// one primary". It stops as soon as a second arrives rather than
	// counting the rest.
	type node struct{ Name, Role string }
	isPrimary := func(n node) bool { return n.Role == "primary" }

	healthy := catena.Of(node{"db-1", "primary"}, node{"db-2", "replica"})
	splitBrain := catena.Of(node{"db-1", "primary"}, node{"db-2", "primary"})

	primary, ok := healthy.Filter(isPrimary).Single()
	_, unambiguous := splitBrain.Filter(isPrimary).Single()
	fmt.Println(primary.Name, ok, unambiguous)
	// Output: db-1 true false
}

func ExampleSeq_ElementAt() {
	// Zero-based. An index out of range, or negative, reports false
	// rather than panicking — there is no counting from the end.
	leaderboard := catena.Of("mira", "tom", "ada")
	runnerUp, ok := leaderboard.ElementAt(1)
	_, fromEnd := leaderboard.ElementAt(-1)
	fmt.Println(runnerUp, ok, fromEnd)
	// Output: tom true false
}

func ExampleSeq_Find() {
	// The first request that broke the 500ms budget; nothing after it is
	// examined.
	type request struct {
		Path string
		Ms   int
	}
	requests := catena.Of(request{"/home", 80}, request{"/search", 740}, request{"/export", 2100})
	slow, ok := requests.Find(func(r request) bool { return r.Ms > 500 })
	fmt.Println(slow.Path, ok)
	// Output: /search true
}

func ExampleSeq_FindLast() {
	// The most recent release that passed — the one to roll back to.
	type deploy struct {
		Version string
		Healthy bool
	}
	history := catena.Of(deploy{"v41", true}, deploy{"v42", true}, deploy{"v43", false})
	good, ok := history.FindLast(func(d deploy) bool { return d.Healthy })
	fmt.Println(good.Version, ok)
	// Output: v42 true
}

func ExampleSeq_FindIndex() {
	// Where the first match is, counted from zero; -1 when there is none.
	log := catena.Of("INFO boot", "INFO listening on :8080", "ERROR disk full")
	hasLevel := func(level string) func(string) bool {
		return func(line string) bool { return strings.HasPrefix(line, level) }
	}
	fmt.Println(log.FindIndex(hasLevel("ERROR")))
	fmt.Println(log.FindIndex(hasLevel("FATAL")))
	// Output:
	// 2
	// -1
}

func ExampleSeq_FindMap() {
	// Fused find and map: the parsed value is returned, not the header
	// line it came from, and the lines after it are never parsed.
	headers := catena.Of("Host: example.com", "Content-Length: 348", "Accept: */*")
	length, ok := headers.FindMap(func(h string) (int, bool) {
		value, found := strings.CutPrefix(h, "Content-Length: ")
		if !found {
			return 0, false
		}
		n, err := strconv.Atoi(value)
		return n, err == nil
	})
	fmt.Println(length, ok)
	// Output: 348 true
}

func ExampleSeq_Any() {
	// Stops at the first match, so it terminates even on an endless
	// source. Here the two lines after the error are never read.
	read := 0
	log := catena.Of("INFO boot", "ERROR disk full", "INFO retrying", "INFO ok").
		OnEach(func(string) { read++ })

	failed := log.Any(func(line string) bool { return strings.HasPrefix(line, "ERROR") })
	fmt.Println(failed, read)
	// Output: true 2
}

func ExampleSeq_All() {
	// Vacuously true on an empty sequence: a pipeline with no checks
	// "passes", so guard for that separately if it matters.
	type check struct {
		Name   string
		Passed bool
	}
	passed := func(c check) bool { return c.Passed }

	ci := catena.Of(check{"lint", true}, check{"unit", true}, check{"e2e", true})
	fmt.Println(ci.All(passed))
	fmt.Println(catena.Empty[check]().All(passed))
	// Output:
	// true
	// true
}

func ExampleSeq_None() {
	// "Did anything go wrong?" asked the other way round; like Any, it
	// stops at the first match.
	statuses := catena.Of(200, 204, 301, 404)
	fmt.Println(statuses.None(func(code int) bool { return code >= 500 }))
	// Output: true
}

func ExampleSeq_Count() {
	// Counts by consuming: nothing is collected along the way.
	body := "the quick brown fox jumps over the lazy dog"
	fmt.Println(catena.FromSlice(strings.Fields(body)).Count())
	// Output: 9
}

func ExampleSeq_CountWhere() {
	// Fused filter and count: one stage rather than two.
	statuses := catena.Of(200, 503, 200, 404, 500, 200)
	fmt.Println(statuses.CountWhere(func(code int) bool { return code >= 500 }))
	// Output: 2
}

func ExampleSeq_IsEmpty() {
	// Answers by consuming one element — on a single-pass source that
	// element is gone.
	statuses := catena.Of(200, 204, 301)
	serverErrors := statuses.Filter(func(code int) bool { return code >= 500 })
	fmt.Println(serverErrors.IsEmpty(), statuses.IsEmpty())
	// Output: true false
}

func ExampleContains() {
	// Membership by ==, stopping at the first hit.
	roles := catena.Of("viewer", "editor", "admin")
	fmt.Println(catena.Contains(roles, "admin"), catena.Contains(roles, "owner"))
	// Output: true false
}

func ExampleIndexOf() {
	// The position of the first equal element, or -1.
	stages := catena.Of("build", "test", "deploy")
	fmt.Println(catena.IndexOf(stages, "deploy"))
	fmt.Println(catena.IndexOf(stages, "rollback"))
	// Output:
	// 2
	// -1
}

func ExampleEqual() {
	// Same elements in the same order. A sequence that merely starts
	// the same way is not equal.
	expected := catena.Of("001_users", "002_orders", "003_indexes")
	applied := catena.Of("001_users", "002_orders", "003_indexes")
	behind := catena.Of("001_users", "002_orders")
	fmt.Println(catena.Equal(expected, applied))
	fmt.Println(catena.Equal(expected, behind))
	// Output:
	// true
	// false
}

// ---- folding and grouping ----

func ExampleSeq_Fold() {
	// The seed is where the result starts: an opening balance of 100,
	// then the day's deposits and withdrawals.
	ledger := catena.Of(250, -75, -20)
	balance := ledger.Fold(100, func(balance, amount int) int { return balance + amount })
	fmt.Println(balance)
	// Output: 255
}

func ExampleSeq_FoldIndexed() {
	// The fold knows where it is: gather the line number of every
	// failing package.
	results := catena.Of("ok   pkg/api", "FAIL pkg/db", "ok   pkg/web", "FAIL pkg/auth")
	failedAt := results.FoldIndexed([]int(nil), func(i int, lines []int, line string) []int {
		if strings.HasPrefix(line, "FAIL") {
			return append(lines, i+1)
		}
		return lines
	})
	fmt.Println(failedAt)
	// Output: [2 4]
}

func ExampleSeq_FoldWhile() {
	// Stops when the callback says so; the accumulator from the stopping
	// call is included. The file that tips the batch past 10 MB is
	// counted, and the one after it is never looked at.
	fileMB := catena.Of(4, 3, 5, 2)
	batchMB := fileMB.FoldWhile(0, func(total, size int) (int, bool) {
		total += size
		return total, total < 10
	})
	fmt.Println(batchMB)
	// Output: 12
}

func ExampleSeq_FoldErr() {
	// Stops at the first error and returns the accumulator so far: the
	// total of the rows that did parse, and why the next one did not.
	amounts := catena.Of("12", "30", "n/a", "7")
	total, err := amounts.FoldErr(0, func(sum int, field string) (int, error) {
		n, err := strconv.Atoi(field)
		if err != nil {
			return 0, err
		}
		return sum + n, nil
	})
	fmt.Println(total, err)
	// Output: 42 strconv.Atoi: parsing "n/a": invalid syntax
}

func ExampleSeq_FoldBy() {
	// Streaming aggregation per key. GroupBy would retain every sale;
	// this retains one running total per region.
	type sale struct {
		Region string
		Amount int
	}
	sales := catena.Of(
		sale{"west", 100}, sale{"east", 40}, sale{"west", 20},
	)
	totals := sales.FoldBy(
		func(s sale) string { return s.Region },
		func(string) int { return 0 },
		func(sum int, s sale) int { return sum + s.Amount },
	)
	fmt.Println(totals["west"], totals["east"])
	// Output: 120 40
}

func ExampleSeq_Reduce() {
	// Uses the first element as the seed, so it suits values with no
	// natural zero — and reports false on empty rather than inventing one.
	type window struct{ From, To int }
	merge := func(a, b window) window {
		return window{min(a.From, b.From), max(a.To, b.To)}
	}
	outages := catena.Of(window{13, 14}, window{9, 11}, window{16, 17})

	span, ok := outages.Reduce(merge)
	_, nonEmpty := catena.Empty[window]().Reduce(merge)
	fmt.Println(span, ok, nonEmpty)
	// Output: {9 17} true false
}

func ExampleSeq_FirstOr() {
	// First, for use inside an expression: no second result to check.
	ports := catena.Of(8080, 8081)
	var none catena.Seq[int]
	fmt.Println(ports.FirstOr(80), none.FirstOr(80))
	// Output: 8080 80
}

func ExampleSeq_LastOr() {
	versions := catena.Of("v1", "v2", "v3")
	fmt.Println(versions.LastOr("v0"), catena.Empty[string]().LastOr("v0"))
	// Output: v3 v0
}

func ExampleSeq_Sample() {
	// Five of a million, each equally likely, holding five in memory.
	// A seeded generator makes the choice repeatable; pass nil for the
	// shared one.
	rng := rand.New(rand.NewPCG(1, 2))
	fmt.Println(len(catena.Range(0, 1_000_000, 1).Sample(5, rng)))

	// A short sequence is returned whole, in order.
	fmt.Println(catena.Of("a", "b").Sample(5, rng))
	// Output:
	// 5
	// [a b]
}

func ExampleSeq_FoldedBy() {
	// FoldBy that stays in the chain: keys come out in the order they
	// were first seen, so the result is deterministic and keeps flowing.
	type hit struct {
		Path  string
		Bytes int
	}
	hits := catena.Of(hit{"/checkout", 900}, hit{"/home", 2400}, hit{"/health", 2}, hit{"/checkout", 1100})
	hits.
		FoldedBy(
			func(h hit) string { return h.Path },
			func(string) int { return 0 },
			func(sum int, h hit) int { return sum + h.Bytes },
		).
		Filter(func(_ string, bytes int) bool { return bytes >= 1000 }).
		ForEach(func(path string, bytes int) { fmt.Println(path, bytes) })
	// Output:
	// /checkout 2000
	// /home 2400
}

func ExampleGroupedBy() {
	// GroupBy in first-seen key order, as a sequence of pairs rather
	// than a map — so it can be ranged over and printed as it stands.
	files := catena.Of("main.go", "README.md", "server.go", "deploy.yaml")
	for ext, names := range catena.GroupedBy(files, filepath.Ext) {
		fmt.Println(ext, names)
	}
	// Output:
	// .go [main.go server.go]
	// .md [README.md]
	// .yaml [deploy.yaml]
}

func ExampleSeq_GroupBy() {
	// Retains every element. For an aggregate, FoldBy is bounded by keys.
	type order struct {
		Customer string
		ID       int
	}
	orders := catena.Of(order{"acme", 1001}, order{"globex", 1002}, order{"acme", 1003})
	byCustomer := orders.GroupBy(func(o order) string { return o.Customer })
	fmt.Println(len(byCustomer["acme"]), byCustomer["globex"])
	// Output: 2 [{globex 1002}]
}

func ExampleSeq_IndexBy() {
	// A lookup table; on a duplicate key the last element wins — which
	// is what defaults followed by overrides want.
	type setting struct{ Key, Value string }
	config := catena.Of(
		setting{"port", "8080"}, setting{"log", "info"}, // defaults
		setting{"port", "9090"}, // override
	).IndexBy(func(s setting) string { return s.Key })
	fmt.Println(config["port"].Value, config["log"].Value)
	// Output: 9090 info
}

func ExampleSeq_TallyBy() {
	// A count per key, with no group ever built: responses by class.
	statuses := catena.Of(200, 503, 201, 404, 500, 200)
	byClass := statuses.TallyBy(func(code int) string { return fmt.Sprintf("%dxx", code/100) })
	fmt.Println(byClass["2xx"], byClass["4xx"], byClass["5xx"])
	// Output: 3 1 2
}

func ExampleSeq_Associate() {
	// Each element supplies both the key and the value.
	env := catena.Of("HOME=/home/ada", "SHELL=/bin/zsh", "LANG=en_GB.UTF-8").
		Associate(func(line string) (string, string) {
			name, value, _ := strings.Cut(line, "=")
			return name, value
		})
	fmt.Println(env["SHELL"], env["LANG"])
	// Output: /bin/zsh en_GB.UTF-8
}

func ExampleSeq_Partition() {
	// Both sides in one pass, each in encounter order.
	signups := catena.Of("ada@example.com", "bob", "eve@example.org", "n/a")
	valid, rejected := signups.Partition(func(email string) bool {
		return strings.Contains(email, "@")
	})
	fmt.Println(valid, rejected)
	// Output: [ada@example.com eve@example.org] [bob n/a]
}

func ExampleAssociateWith() {
	// The elements become the keys; the function supplies each value.
	hosts := catena.Of("web-1", "db-1")
	probes := catena.AssociateWith(hosts, func(host string) string {
		return "https://" + host + ".internal/healthz"
	})
	fmt.Println(probes["db-1"])
	// Output: https://db-1.internal/healthz
}

func ExampleTally() {
	// How many times each value occurs.
	methods := catena.Of("GET", "POST", "GET", "GET", "DELETE")
	counts := catena.Tally(methods)
	fmt.Println(counts["GET"], counts["POST"], counts["PUT"])
	// Output: 3 1 0
}

func ExampleToKeySet() {
	// Duplicates collapse into a set built for membership tests.
	blocked := catena.ToKeySet(catena.Of("10.0.0.7", "10.0.0.9", "10.0.0.7"))
	_, denied := blocked["10.0.0.9"]
	fmt.Println(len(blocked), denied)
	// Output: 2 true
}

// ---- aggregating ----

func ExampleSeq_MaxBy() {
	// Returns the ELEMENT with the largest key; ties go to the first.
	type endpoint struct {
		Path  string
		P99ms int
	}
	endpoints := catena.Of(endpoint{"/home", 120}, endpoint{"/search", 840}, endpoint{"/export", 840})
	slowest, _ := endpoints.MaxBy(func(e endpoint) int { return e.P99ms })
	fmt.Println(slowest.Path)
	// Output: /search
}

func ExampleSeq_MinBy() {
	// The whole quote comes back, not just its price.
	type quote struct {
		Carrier string
		Cents   int
	}
	quotes := catena.Of(quote{"dhl", 1250}, quote{"ups", 990}, quote{"fedex", 1400})
	cheapest, _ := quotes.MinBy(func(q quote) int { return q.Cents })
	fmt.Println(cheapest.Carrier)
	// Output: ups
}

func ExampleSeq_MaxOf() {
	// Returns the KEY, where MaxBy returns the element — the -By/-Of
	// distinction, which holds across the whole library.
	type file struct {
		Name  string
		Bytes int64
	}
	files := catena.Of(file{"app.log", 52_000}, file{"core.dump", 9_800_000}, file{"go.mod", 310})
	largest, _ := files.MaxOf(func(f file) int64 { return f.Bytes })
	fmt.Println(largest)
	// Output: 9800000
}

func ExampleSeq_MinOf() {
	// The lowest stock level, whichever product it belongs to.
	type product struct {
		SKU     string
		InStock int
	}
	shelf := catena.Of(product{"mug", 14}, product{"tee", 3}, product{"cap", 27})
	lowest, _ := shelf.MinOf(func(p product) int { return p.InStock })
	fmt.Println(lowest)
	// Output: 3
}

func ExampleSeq_MinMaxOf() {
	// Both ends in a single pass.
	type reading struct {
		Sensor string
		TempC  float64
	}
	readings := catena.Of(reading{"attic", 31.5}, reading{"cellar", 12.0}, reading{"hall", 20.5})
	coldest, warmest, ok := readings.MinMaxOf(func(r reading) float64 { return r.TempC })
	fmt.Println(coldest, warmest, ok)
	// Output: 12 31.5 true
}

func ExampleSeq_MaxWith() {
	// A comparator, for orderings no single key expresses: major first,
	// then minor.
	type version struct{ Major, Minor int }
	tags := catena.Of(version{1, 12}, version{2, 3}, version{2, 0})
	latest, _ := tags.MaxWith(func(a, b version) int {
		return cmp.Or(cmp.Compare(a.Major, b.Major), cmp.Compare(a.Minor, b.Minor))
	})
	fmt.Println(latest)
	// Output: {2 3}
}

func ExampleSeq_MinWith() {
	// What to work on next: the most urgent ticket, and among equally
	// urgent ones the oldest.
	type ticket struct {
		ID       string
		Priority int // 1 is most urgent
		AgeDays  int
	}
	backlog := catena.Of(ticket{"T-7", 2, 30}, ticket{"T-8", 1, 2}, ticket{"T-9", 1, 6})
	next, _ := backlog.MinWith(func(a, b ticket) int {
		return cmp.Or(cmp.Compare(a.Priority, b.Priority), cmp.Compare(b.AgeDays, a.AgeDays))
	})
	fmt.Println(next.ID)
	// Output: T-9
}

func ExampleSeq_TopNBy() {
	// A bounded heap of n, not a sort: memory is O(n) rather than O(all),
	// which is the difference between a working pipeline and an OOM on a
	// large scan. Output is sorted descending, ties in encounter order.
	type customer struct {
		Name  string
		Spend int
	}
	customers := catena.Of(
		customer{"acme", 5200}, customer{"initech", 800}, customer{"globex", 9100},
		customer{"hooli", 5200}, customer{"umbrella", 300},
	)
	fmt.Println(customers.TopNBy(3, func(c customer) int { return c.Spend }))
	// Output: [{globex 9100} {acme 5200} {hooli 5200}]
}

func ExampleSeq_BottomNBy() {
	// The same bounded heap at the other end, sorted ascending: the two
	// least-loaded servers to send new work to.
	type server struct {
		Host string
		Load int
	}
	pool := catena.Of(server{"web-1", 71}, server{"web-2", 12}, server{"web-3", 94}, server{"web-4", 35})
	fmt.Println(pool.BottomNBy(2, func(s server) int { return s.Load }))
	// Output: [{web-2 12} {web-4 35}]
}

func ExampleSeq_SumOf() {
	// The selector computes what to add, so a basket totals without an
	// intermediate Map.
	type line struct {
		Qty, UnitCents int
	}
	basket := catena.Of(line{2, 1250}, line{1, 400}, line{3, 99})
	fmt.Println(basket.SumOf(func(l line) int { return l.Qty * l.UnitCents }))
	// Output: 3197
}

func ExampleSeq_ProductOf() {
	// A request that needs every dependency is up only as often as all
	// of them are. The empty product is 1, the multiplicative identity:
	// nothing to depend on, nothing to fail.
	type dependency struct {
		Name   string
		Uptime float64
	}
	uptime := func(d dependency) float64 { return d.Uptime }
	deps := catena.Of(dependency{"db", 0.999}, dependency{"cache", 0.995}, dependency{"auth", 0.99})
	fmt.Printf("%.3f\n", deps.ProductOf(uptime))
	fmt.Printf("%.3f\n", catena.Empty[dependency]().ProductOf(uptime))
	// Output:
	// 0.984
	// 1.000
}

func ExampleSeq_AverageOf() {
	// Accumulates in float64, so integer latencies give a fractional
	// mean — and reports false on empty rather than dividing by zero.
	type request struct {
		Path string
		Ms   int
	}
	ms := func(r request) int { return r.Ms }
	requests := catena.Of(request{"/home", 120}, request{"/cart", 85}, request{"/search", 310})
	mean, ok := requests.AverageOf(ms)
	_, nonEmpty := catena.Empty[request]().AverageOf(ms)
	fmt.Printf("%.2f %v %v\n", mean, ok, nonEmpty)
	// Output: 171.67 true false
}

func ExampleSeq_JoinToString() {
	// Map and join in one step: the To line of an email.
	type user struct{ Name, Email string }
	team := catena.Of(user{"Ada", "ada@example.com"}, user{"Bob", "bob@example.com"})
	fmt.Println(team.JoinToString(", ", func(u user) string {
		return fmt.Sprintf("%s <%s>", u.Name, u.Email)
	}))
	// Output: Ada <ada@example.com>, Bob <bob@example.com>
}

func ExampleMax() {
	// The no-selector form of MaxOf, for values that order themselves.
	latenciesMs := catena.Of(120, 85, 310, 96)
	worst, ok := catena.Max(latenciesMs)
	fmt.Println(worst, ok)
	// Output: 310 true
}

func ExampleMin() {
	// False on empty, since there is no smallest of nothing.
	prices := catena.Of(12.50, 9.99, 14.00)
	cheapest, ok := catena.Min(prices)
	_, nonEmpty := catena.Min(catena.Empty[float64]())
	fmt.Println(cheapest, ok, nonEmpty)
	// Output: 9.99 true false
}

func ExampleMinMax() {
	// The range of a day's readings in one pass rather than two.
	tempsC := catena.Of(14.5, 21.0, 9.5, 17.0)
	low, high, ok := catena.MinMax(tempsC)
	fmt.Println(low, high, ok)
	// Output: 9.5 21 true
}

func ExampleTopN() {
	// The podium, best first, without sorting the whole field.
	scores := catena.Of(71, 94, 58, 88, 90)
	fmt.Println(catena.TopN(scores, 3))
	// Output: [94 90 88]
}

func ExampleBottomN() {
	// The two fastest laps, quickest first.
	lapSecs := catena.Of(92.4, 89.7, 95.1, 90.2)
	fmt.Println(catena.BottomN(lapSecs, 2))
	// Output: [89.7 90.2]
}

func ExampleSum() {
	// Integer overflow wraps, exactly as + does.
	downloadsPerDay := catena.Of(1_204, 980, 1_532)
	fmt.Println(catena.Sum(downloadsPerDay))
	// Output: 3716
}

func ExampleProduct() {
	// A parcel's volume from its sides, in cubic centimetres.
	sidesCm := catena.Of(30, 20, 15)
	fmt.Println(catena.Product(sidesCm))
	// Output: 9000
}

func ExampleAverage() {
	// Always a float64, whatever the element type; false on empty.
	ratings := catena.Of(5, 4, 4, 2)
	mean, ok := catena.Average(ratings)
	fmt.Println(mean, ok)
	// Output: 3.75 true
}

func ExampleJoin() {
	// strings.Join for a sequence, with no slice in between.
	segments := catena.Of("api", "v2", "orders")
	fmt.Println("/" + catena.Join(segments, "/"))
	// Output: /api/v2/orders
}
