package catena_test

// Examples for transforming, combining, ordering and batching. See
// example_filtering_test.go for why these are Example functions.

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/NerdMeNot/catena"
)

// ---- transforming ----

func ExampleSeq_Map() {
	// The element type changes mid-chain, which is what generic methods
	// made possible.
	fmt.Println(catena.Of(1, 2, 3).
		Map(func(n int) string { return strings.Repeat("*", n) }).
		Collect())
	// Output: [* ** ***]
}

func ExampleSeq_MapIndexed() {
	fmt.Println(catena.Of("a", "b", "c").
		MapIndexed(func(i int, s string) string { return fmt.Sprintf("%d:%s", i, s) }).
		Collect())
	// Output: [0:a 1:b 2:c]
}

func ExampleSeq_FilterMap() {
	// Fused filter and map: one stage, and the comma-ok shape means the
	// mapped value is discarded rather than computed twice.
	fmt.Println(catena.Of("1", "x", "3").
		FilterMap(func(s string) (int, bool) {
			n, err := strconv.Atoi(s)
			return n, err == nil
		}).
		Collect())
	// Output: [1 3]
}

func ExampleSeq_FlatMap() {
	fmt.Println(catena.Of(1, 2).
		FlatMap(func(n int) catena.Seq[int] { return catena.Of(n, -n) }).
		Collect())
	// Output: [1 -1 2 -2]
}

func ExampleSeq_FlatMapSlice() {
	// The same, when the callback already has a slice in hand.
	fmt.Println(catena.Of("a b", "c d").
		FlatMapSlice(func(s string) []string { return strings.Fields(s) }).
		Collect())
	// Output: [a b c d]
}

func ExampleSeq_Scan() {
	// A running fold: the accumulator is emitted at each step. The
	// initial value itself is not emitted, so the output is as long as
	// the input.
	fmt.Println(catena.Of(1, 2, 3, 4).
		Scan(0, func(sum, n int) int { return sum + n }).
		Collect())
	// Output: [1 3 6 10]
}

func ExampleSeq_MapErr() {
	// A mapping that can fail produces a Try; a failed call yields the
	// zero value alongside the error, never a half-built one.
	parsed := catena.Of("1", "two", "3").MapErr(strconv.Atoi)
	vals, errs := parsed.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [1 3] 1
}

func ExampleSeq_OnEach() {
	// Side effects without changing the stream — logging, metrics,
	// progress. Elements pass through untouched.
	seen := 0
	total := catena.Of(1, 2, 3).
		OnEach(func(int) { seen++ }).
		SumOf(catena.Self[int])
	fmt.Println(total, seen)
	// Output: 6 3
}

func ExampleFlatten() {
	inner := catena.Of(catena.Of(1, 2), catena.Of(3))
	fmt.Println(catena.Flatten(inner).Collect())
	// Output: [1 2 3]
}

func ExampleFlattenSlices() {
	fmt.Println(catena.FlattenSlices(catena.Of([]int{1, 2}, []int{3})).Collect())
	// Output: [1 2 3]
}

// ---- combining ----

func ExampleSeq_Concat() {
	fmt.Println(catena.Of(1, 2).Concat(catena.Of(3), catena.Of(4, 5)).Collect())
	// Output: [1 2 3 4 5]
}

func ExampleSeq_Append() {
	fmt.Println(catena.Of("a").Append("b", "c").Collect())
	// Output: [a b c]
}

func ExampleSeq_Prepend() {
	fmt.Println(catena.Of("c").Prepend("a", "b").Collect())
	// Output: [a b c]
}

func ExampleChain() {
	// The variadic form, for when the sequences are in a slice already.
	fmt.Println(catena.Chain(catena.Of(1), catena.Of(2), catena.Of(3)).Collect())
	// Output: [1 2 3]
}

func ExampleSeq_Zip() {
	// Pairs elements positionally and stops at the shorter side.
	names := catena.Of("ada", "bob", "eve")
	scores := catena.Of(90, 85)
	fmt.Println(names.Zip(scores).
		MapTo(func(n string, s int) string { return fmt.Sprintf("%s=%d", n, s) }).
		Collect())
	// Output: [ada=90 bob=85]
}

func ExampleSeq_ZipWithNext() {
	// Each element paired with its successor — deltas, gaps, transitions.
	fmt.Println(catena.Of(3, 7, 12).ZipWithNext().
		MapTo(func(a, b int) int { return b - a }).
		Collect())
	// Output: [4 5]
}

func ExampleSeq_WithIndex() {
	fmt.Println(catena.Of("a", "b").WithIndex().
		MapTo(func(i int, s string) string { return fmt.Sprintf("%d%s", i, s) }).
		Collect())
	// Output: [0a 1b]
}

func ExampleSeq_JoinBy() {
	// A relational inner join: unmatched rows on either side are dropped,
	// and duplicate keys produce the cross product.
	type order struct {
		Customer int
		Amount   int
	}
	type customer struct {
		ID   int
		Name string
	}
	orders := catena.Of(order{1, 30}, order{2, 10}, order{9, 99})
	customers := catena.Of(customer{1, "ada"}, customer{2, "bob"})

	fmt.Println(orders.JoinBy(customers,
		func(o order) int { return o.Customer },
		func(c customer) int { return c.ID },
		func(o order, c customer) string { return fmt.Sprintf("%s:%d", c.Name, o.Amount) },
	).Collect())
	// Output: [ada:30 bob:10]
}

func ExampleUnion() {
	// Set semantics: the result is deduplicated, in encounter order,
	// left operand first.
	fmt.Println(catena.Union(catena.Of(1, 2, 2), catena.Of(3, 1)).Collect())
	// Output: [1 2 3]
}

func ExampleIntersect() {
	fmt.Println(catena.Intersect(catena.Of(1, 2, 3), catena.Of(3, 1)).Collect())
	// Output: [1 3]
}

func ExampleExcept() {
	fmt.Println(catena.Except(catena.Of(1, 2, 3), catena.Of(2)).Collect())
	// Output: [1 3]
}

func ExampleSeq_Intersperse() {
	fmt.Println(catena.Of("a", "b", "c").Intersperse("-").Collect())
	// Output: [a - b - c]
}

func ExampleSeq_IfEmpty() {
	// A fallback for the whole sequence, not per element.
	fmt.Println(catena.Of(1, 2).IfEmpty(0).Collect())
	fmt.Println(catena.Empty[int]().IfEmpty(0).Collect())
	// Output:
	// [1 2]
	// [0]
}

// ---- ordering ----

func ExampleSorted() {
	fmt.Println(catena.Sorted(catena.Of(3, 1, 2)).Collect())
	// Output: [1 2 3]
}

func ExampleSortedDesc() {
	fmt.Println(catena.SortedDesc(catena.Of(3, 1, 2)).Collect())
	// Output: [3 2 1]
}

func ExampleSeq_SortedBy() {
	// Stable, and the selector runs exactly once per element rather than
	// once per comparison — so an expensive key is affordable. Stability
	// shows here: kiwi and date are both 4 long, and kiwi came first.
	words := catena.Of("kiwi", "fig", "banana", "date")
	fmt.Println(words.SortedBy(func(s string) int { return len(s) }).Collect())
	// Output: [fig kiwi date banana]
}

func ExampleSeq_SortedByDesc() {
	words := catena.Of("kiwi", "fig", "banana")
	fmt.Println(words.SortedByDesc(func(s string) int { return len(s) }).Collect())
	// Output: [banana kiwi fig]
}

func ExampleSeq_SortedWith() {
	// A comparator, for orderings a single key cannot express.
	fmt.Println(catena.Of("b", "A", "c").
		SortedWith(func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) }).
		Collect())
	// Output: [A b c]
}

func ExampleSeq_Reversed() {
	fmt.Println(catena.Of(1, 2, 3).Reversed().Collect())
	// Output: [3 2 1]
}

// ---- batching ----

func ExampleChunked() {
	// Fixed-size batches; the last one may be short. Every chunk is a
	// fresh slice, so keeping one is safe.
	for c := range catena.Chunked(catena.Range(1, 8, 1), 3).Seq() {
		fmt.Println(c)
	}
	// Output:
	// [1 2 3]
	// [4 5 6]
	// [7]
}

func ExampleChunkedBy() {
	// A chunk per run of equal keys — the streaming answer to grouping
	// already-sorted input, in memory bounded by the longest run.
	for c := range catena.ChunkedBy(catena.Of(1, 1, 2, 3, 3), catena.Self[int]).Seq() {
		fmt.Println(c)
	}
	// Output:
	// [1 1]
	// [2]
	// [3 3]
}

func ExampleWindowed() {
	// Overlapping windows: size 3 advancing by 1. Trailing elements that
	// cannot fill a window are dropped.
	for w := range catena.Windowed(catena.Range(1, 6, 1), 3, 1).Seq() {
		fmt.Println(w)
	}
	// Output:
	// [1 2 3]
	// [2 3 4]
	// [3 4 5]
}
