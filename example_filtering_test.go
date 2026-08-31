package catena_test

// Examples for the filtering operators. Each is a real test: `go test`
// runs it and diffs the printed output against the `// Output:` comment,
// so an example that stops being true fails the build. The operator
// documentation on the website and on pkg.go.dev is generated from these,
// which is why they are written to be read rather than merely to pass.

import (
	"fmt"
	"strings"

	"github.com/NerdMeNot/catena"
)

func ExampleSeq_Filter() {
	fmt.Println(catena.Of(1, 2, 3, 4, 5).
		Filter(func(n int) bool { return n%2 == 0 }).
		Collect())
	// Output: [2 4]
}

func ExampleSeq_FilterNot() {
	// The negated form, for when the predicate reads better positively.
	fmt.Println(catena.Of("go", "", "rust", "", "zig").
		FilterNot(func(s string) bool { return s == "" }).
		Collect())
	// Output: [go rust zig]
}

func ExampleSeq_FilterIndexed() {
	// The index counts source elements, not surviving ones.
	fmt.Println(catena.Of("a", "b", "c", "d", "e").
		FilterIndexed(func(i int, _ string) bool { return i%2 == 0 }).
		Collect())
	// Output: [a c e]
}

func ExampleSeq_FilterErr() {
	// A predicate that can fail produces a Try, so the caller chooses
	// what a failure means rather than the pipeline deciding.
	ports := catena.Of("80", "443", "https", "8080")
	valid := ports.FilterErr(func(s string) (bool, error) {
		if strings.ContainsAny(s, "abcdefghijklmnopqrstuvwxyz") {
			return false, fmt.Errorf("not a port: %q", s)
		}
		return len(s) > 2, nil
	})

	kept, errs := valid.CollectAll()
	fmt.Println(kept, errs)
	// Output: [443 8080] [not a port: "https"]
}

func ExampleSeq_DistinctBy() {
	type user struct {
		Org, Name string
	}
	users := catena.Of(
		user{"acme", "ada"},
		user{"globex", "bob"},
		user{"acme", "eve"},
	)

	// One user per org; the first occurrence wins.
	fmt.Println(users.
		DistinctBy(func(u user) string { return u.Org }).
		Collect())
	// Output: [{acme ada} {globex bob}]
}

func ExampleSeq_DistinctWith() {
	// For keys that are not comparable, or an equality of your own.
	// Retains every distinct element and compares against all of them,
	// so this is for small inputs.
	fmt.Println(catena.Of("Go", "GO", "rust", "go").
		DistinctWith(strings.EqualFold).
		Collect())
	// Output: [Go rust]
}

func ExampleSeq_DedupeBy() {
	// Collapses CONSECUTIVE runs only, in O(1) memory — the streaming
	// alternative to DistinctBy, and the right choice on an unbounded
	// source where a seen-set would grow forever.
	type reading struct {
		Tick int
		Zone string
	}
	readings := catena.Of(
		reading{1, "cold"}, reading{2, "cold"},
		reading{3, "warm"}, reading{4, "cold"},
	)

	fmt.Println(readings.
		DedupeBy(func(r reading) string { return r.Zone }).
		Collect())
	// Output: [{1 cold} {3 warm} {4 cold}]
}

func ExampleDistinct() {
	// Distinct constrains the element type, so it is a package function
	// rather than a method — a method on Seq[T any] may require nothing
	// of T. First occurrence wins, and encounter order is preserved.
	fmt.Println(catena.Distinct(catena.Of(3, 1, 3, 2, 1)).Collect())
	// Output: [3 1 2]
}

func ExampleDedupe() {
	// Consecutive duplicates only. On sorted input it equals Distinct at
	// a fraction of the cost; on unsorted input the two differ.
	fmt.Println(catena.Dedupe(catena.Of(3, 3, 1, 1, 3)).Collect())
	// Output: [3 1 3]
}

func ExampleNonZero() {
	// Drops the zero value of T — empty strings here, but equally 0,
	// nil pointers, or a zero struct.
	fmt.Println(catena.NonZero(catena.Of("go", "", "rust", "")).Collect())
	// Output: [go rust]
}
