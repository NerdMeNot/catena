package catena_test

// Examples for the filtering operators. Each is a real test: `go test`
// runs it and diffs the printed output against the `// Output:` comment,
// so an example that stops being true fails the build. The operator
// documentation on the website and on pkg.go.dev is generated from these,
// which is why they are written to be read rather than merely to pass.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/NerdMeNot/catena"
)

func ExampleSeq_Filter() {
	// The unpaid orders worth a reminder email. Nothing is tested until
	// Collect asks for the result.
	type order struct {
		ID    string
		Total float64
		Paid  bool
	}
	orders := catena.Of(
		order{"A-1001", 42.50, true},
		order{"A-1002", 310.00, false},
		order{"A-1003", 18.99, false},
		order{"A-1004", 129.00, false},
	)

	fmt.Println(orders.
		Filter(func(o order) bool { return !o.Paid && o.Total >= 100 }).
		Map(func(o order) string { return o.ID }).
		Collect())
	// Output: [A-1002 A-1004]
}

func ExampleSeq_FilterNot() {
	// The negated form, for when the predicate reads better positively:
	// "is a probe" is the natural question, and the traffic report wants
	// everything that is not one.
	isProbe := func(path string) bool {
		return path == "/healthz" || path == "/metrics"
	}
	paths := catena.Of("/checkout", "/healthz", "/cart", "/metrics", "/login")

	fmt.Println(paths.FilterNot(isProbe).Collect())
	// Output: [/checkout /cart /login]
}

func ExampleSeq_FilterIndexed() {
	// The index is the line's position in the source, not among the
	// survivors: line 0 is the header however many blank lines the
	// predicate has dropped along the way.
	export := catena.Of("host,port", "", "db-1,5432", "", "cache-1,6379")

	fmt.Println(export.
		FilterIndexed(func(i int, line string) bool { return i > 0 && line != "" }).
		Collect())
	// Output: [db-1,5432 cache-1,6379]
}

func ExampleSeq_FilterErr() {
	// A predicate that can fail produces a Try, so the caller chooses
	// what a failure means rather than the pipeline deciding. Here a bad
	// config entry is reported alongside the ports that did parse.
	configured := catena.Of("8080", "443", "http", "22")
	privileged := configured.FilterErr(func(s string) (bool, error) {
		port, err := strconv.Atoi(s)
		return port < 1024, err
	})

	kept, errs := privileged.CollectAll()
	fmt.Println(kept, errs)
	// Output: [443 22] [strconv.Atoi: parsing "http": invalid syntax]
}

func ExampleSeq_DistinctBy() {
	// Page the on-call engineer once per host, not once per alert. The
	// first alert seen for a host is the one that survives.
	type alert struct {
		Host, Problem string
	}
	alerts := catena.Of(
		alert{"db-1", "disk 91% full"},
		alert{"web-3", "5xx rate high"},
		alert{"db-1", "replication lag"},
	)

	fmt.Println(alerts.
		DistinctBy(func(a alert) string { return a.Host }).
		Collect())
	// Output: [{db-1 disk 91% full} {web-3 5xx rate high}]
}

func ExampleSeq_DistinctWith() {
	// For an equality of your own — here, signups that differ only in
	// letter case are the same person — or for keys that are not
	// comparable. Retains every distinct element and compares against
	// all of them, so this is for small inputs.
	signups := catena.Of(
		"Ada@example.com", "bob@example.com", "ada@example.com", "ADA@EXAMPLE.COM",
	)

	fmt.Println(signups.DistinctWith(strings.EqualFold).Collect())
	// Output: [Ada@example.com bob@example.com]
}

func ExampleSeq_DedupeBy() {
	// Turns a stream of health checks into a stream of state changes.
	// Only CONSECUTIVE repeats collapse, in O(1) memory, so the second
	// "up" is kept — and it is safe on an unbounded source, where
	// DistinctBy's seen-set would grow forever.
	type check struct {
		At, Status string
	}
	checks := catena.Of(
		check{"09:00", "up"}, check{"09:01", "up"},
		check{"09:02", "down"}, check{"09:03", "down"},
		check{"09:04", "up"},
	)

	fmt.Println(checks.
		DedupeBy(func(c check) string { return c.Status }).
		Collect())
	// Output: [{09:00 up} {09:02 down} {09:04 up}]
}

func ExampleDistinct() {
	// The unique client addresses in an access log, in the order they
	// first appeared. Distinct constrains the element type, so it is a
	// package function rather than a method — a method on Seq[T any] may
	// require nothing of T.
	clients := catena.Of("10.0.0.7", "10.0.0.9", "10.0.0.7", "10.0.0.4", "10.0.0.9")

	fmt.Println(catena.Distinct(clients).Collect())
	// Output: [10.0.0.7 10.0.0.9 10.0.0.4]
}

func ExampleDedupe() {
	// Like uniq(1): consecutive duplicates only, so a repeat that comes
	// back later is kept. On sorted input it equals Distinct at a
	// fraction of the cost; on unsorted input the two differ.
	log := catena.Of(
		"connection reset", "connection reset", "connection reset",
		"reconnected",
		"connection reset",
	)

	fmt.Println(catena.Dedupe(log).JoinToString(" | ", catena.Self[string]))
	// Output: connection reset | reconnected | connection reset
}

func ExampleNonZero() {
	// Drops the zero value of T — the empty strings that splitting a
	// path leaves behind here, but equally 0, nil pointers, or a zero
	// struct.
	segments := catena.FromSlice(strings.Split("/var/log//nginx/", "/"))

	fmt.Println(catena.NonZero(segments).Collect())
	// Output: [var log nginx]
}
