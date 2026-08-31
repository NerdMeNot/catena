package catena_test

// Examples for consuming, searching, folding and aggregating. See
// example_filtering_test.go for why these are Example functions.

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/NerdMeNot/catena"
)

// ---- consuming ----

func ExampleSeq_Collect() {
	// nil for an empty sequence, matching slices.Collect.
	fmt.Println(catena.Of(1, 2).Collect(), catena.Empty[int]().Collect() == nil)
	// Output: [1 2] true
}

func ExampleSeq_ToList() {
	// The eager twin: a List has the same operations, evaluated at once.
	l := catena.Of(3, 1, 2).ToList()
	fmt.Println(l.Len(), l.At(0))
	// Output: 3 3
}

func ExampleSeq_Seq() {
	// A free conversion to the standard iterator type, in both
	// directions — Seq IS iter.Seq underneath.
	fmt.Println(slices.Collect(catena.Of(1, 2, 3).Seq()))
	// Output: [1 2 3]
}

func ExampleSeq_Pull() {
	// Inverts control for hand-written loops. THE CALLER MUST CALL stop,
	// or a producer holding a resource never releases it.
	next, stop := catena.Of("a", "b").Pull()
	defer stop()

	v, ok := next()
	fmt.Println(v, ok)
	// Output: a true
}

func ExampleSeq_ToChan() {
	// The fan-out mechanism: a Seq is not safe to consume from two
	// goroutines, a channel is. Cancelling ctx closes the channel.
	for v := range catena.Of(1, 2, 3).ToChan(context.Background()) {
		fmt.Print(v, " ")
	}
	// Output: 1 2 3
}

func ExampleSeq_ForEach() {
	catena.Of("a", "b").ForEach(func(s string) { fmt.Print(s) })
	// Output: ab
}

func ExampleSeq_ForEachIndexed() {
	catena.Of("a", "b").ForEachIndexed(func(i int, s string) { fmt.Printf("%d=%s ", i, s) })
	// Output: 0=a 1=b
}

func ExampleSeq_ForEachErr() {
	// Stops at the first error the callback returns, and returns it.
	err := catena.Of(1, 2, 3).ForEachErr(func(n int) error {
		if n == 2 {
			return errors.New("stopped at 2")
		}
		fmt.Println("handled", n)
		return nil
	})
	fmt.Println("err:", err)
	// Output:
	// handled 1
	// err: stopped at 2
}

func ExampleSeq_Drain() {
	// Consume for side effects alone, discarding the elements.
	count := 0
	catena.Of(1, 2, 3).OnEach(func(int) { count++ }).Drain()
	fmt.Println(count)
	// Output: 3
}

func ExampleSeq_Once() {
	// A development guard for the single-pass contract: the second
	// consumption panics instead of silently re-running the producer.
	s := catena.Of(1, 2).Once()
	fmt.Println(s.Collect())

	defer func() { fmt.Println("recovered:", recover()) }()
	s.Collect()
	// Output:
	// [1 2]
	// recovered: catena: Once: sequence consumed more than once
}

func ExampleSeq_UntilDone() {
	// Cancellation enters at the edge rather than threading a context
	// through every stage. The context's error arrives as an element.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := catena.Of(1, 2, 3).UntilDone(ctx).Collect()
	fmt.Println(err)
	// Output: context canceled
}

// ---- searching and testing ----

func ExampleSeq_First() {
	v, ok := catena.Of(3, 1).First()
	_, empty := catena.Empty[int]().First()
	fmt.Println(v, ok, empty)
	// Output: 3 true false
}

func ExampleSeq_Last() {
	v, ok := catena.Of(3, 1).Last()
	fmt.Println(v, ok)
	// Output: 1 true
}

func ExampleSeq_Single() {
	// True only for exactly one element; it stops as soon as a second
	// arrives rather than counting the rest.
	a, ok1 := catena.Of(7).Single()
	_, ok2 := catena.Of(7, 8).Single()
	fmt.Println(a, ok1, ok2)
	// Output: 7 true false
}

func ExampleSeq_ElementAt() {
	v, ok := catena.Of("a", "b", "c").ElementAt(1)
	_, neg := catena.Of("a").ElementAt(-1)
	fmt.Println(v, ok, neg)
	// Output: b true false
}

func ExampleSeq_Find() {
	v, ok := catena.Of(1, 4, 9).Find(func(n int) bool { return n > 3 })
	fmt.Println(v, ok)
	// Output: 4 true
}

func ExampleSeq_FindLast() {
	v, ok := catena.Of(1, 4, 9).FindLast(func(n int) bool { return n > 3 })
	fmt.Println(v, ok)
	// Output: 9 true
}

func ExampleSeq_FindIndex() {
	fmt.Println(catena.Of("a", "b").FindIndex(func(s string) bool { return s == "b" }))
	fmt.Println(catena.Of("a").FindIndex(func(s string) bool { return s == "z" }))
	// Output:
	// 1
	// -1
}

func ExampleSeq_FindMap() {
	// Fused find and map: the mapped value is returned, not the element.
	v, ok := catena.Of("x", "12", "y").FindMap(func(s string) (int, bool) {
		n := 0
		_, err := fmt.Sscanf(s, "%d", &n)
		return n, err == nil
	})
	fmt.Println(v, ok)
	// Output: 12 true
}

func ExampleSeq_Any() {
	// Stops at the first match, so it terminates on an infinite source.
	fmt.Println(catena.Generate(1, func(n int) int { return n + 1 }).
		Any(func(n int) bool { return n > 100 }))
	// Output: true
}

func ExampleSeq_All() {
	// Vacuously true on an empty sequence.
	fmt.Println(catena.Of(2, 4).All(func(n int) bool { return n%2 == 0 }))
	fmt.Println(catena.Empty[int]().All(func(n int) bool { return false }))
	// Output:
	// true
	// true
}

func ExampleSeq_None() {
	fmt.Println(catena.Of(1, 3).None(func(n int) bool { return n%2 == 0 }))
	// Output: true
}

func ExampleSeq_Count() {
	fmt.Println(catena.Of("a", "b", "c").Count())
	// Output: 3
}

func ExampleSeq_CountWhere() {
	// Fused filter and count: one stage rather than two.
	fmt.Println(catena.Range(1, 11, 1).CountWhere(func(n int) bool { return n%3 == 0 }))
	// Output: 3
}

func ExampleSeq_IsEmpty() {
	// Answers by consuming one element — on a single-pass source that
	// element is gone.
	fmt.Println(catena.Empty[int]().IsEmpty(), catena.Of(1).IsEmpty())
	// Output: true false
}

func ExampleContains() {
	fmt.Println(catena.Contains(catena.Of(1, 2, 3), 2))
	// Output: true
}

func ExampleIndexOf() {
	fmt.Println(catena.IndexOf(catena.Of("a", "b"), "b"))
	fmt.Println(catena.IndexOf(catena.Of("a"), "z"))
	// Output:
	// 1
	// -1
}

func ExampleEqual() {
	fmt.Println(catena.Equal(catena.Of(1, 2), catena.Of(1, 2)))
	fmt.Println(catena.Equal(catena.Of(1, 2), catena.Of(1)))
	// Output:
	// true
	// false
}

// ---- folding and grouping ----

func ExampleSeq_Fold() {
	fmt.Println(catena.Of(1, 2, 3).Fold(100, func(acc, n int) int { return acc + n }))
	// Output: 106
}

func ExampleSeq_FoldIndexed() {
	fmt.Println(catena.Of(10, 20).FoldIndexed(0, func(i, acc, n int) int { return acc + i*n }))
	// Output: 20
}

func ExampleSeq_FoldWhile() {
	// Stops when the callback says so; the accumulator from the stopping
	// call is included.
	fmt.Println(catena.Of(1, 2, 3, 4).FoldWhile(0, func(acc, n int) (int, bool) {
		acc += n
		return acc, acc < 5
	}))
	// Output: 6
}

func ExampleSeq_FoldErr() {
	// Stops at the first error and returns the accumulator so far.
	acc, err := catena.Of(1, 2, 3).FoldErr(0, func(acc, n int) (int, error) {
		if n == 3 {
			return 0, errors.New("three is too many")
		}
		return acc + n, nil
	})
	fmt.Println(acc, err)
	// Output: 3 three is too many
}

func ExampleSeq_FoldBy() {
	// Streaming aggregation per key. GroupBy would retain every element;
	// this retains one accumulator per distinct key.
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
	// Uses the first element as the seed, so it reports false on empty
	// rather than inventing a zero.
	v, ok := catena.Of(3, 1, 2).Reduce(func(a, b int) int { return a * b })
	_, empty := catena.Empty[int]().Reduce(func(a, b int) int { return a })
	fmt.Println(v, ok, empty)
	// Output: 6 true false
}

func ExampleSeq_GroupBy() {
	// Retains every element. For an aggregate, FoldBy is bounded by keys.
	byParity := catena.Range(1, 6, 1).GroupBy(func(n int) string {
		if n%2 == 0 {
			return "even"
		}
		return "odd"
	})
	fmt.Println(byParity["odd"], byParity["even"])
	// Output: [1 3 5] [2 4]
}

func ExampleSeq_IndexBy() {
	// A lookup table; on a duplicate key the last element wins.
	m := catena.Of("apple", "avocado", "blueberry").
		IndexBy(func(s string) byte { return s[0] })
	fmt.Println(string(m['a']), string(m['b']))
	// Output: avocado blueberry
}

func ExampleSeq_TallyBy() {
	counts := catena.Of("apple", "avocado", "blueberry").
		TallyBy(func(s string) byte { return s[0] })
	fmt.Println(counts['a'], counts['b'])
	// Output: 2 1
}

func ExampleSeq_Associate() {
	m := catena.Of(1, 2).Associate(func(n int) (int, string) {
		return n, strings.Repeat("*", n)
	})
	fmt.Println(m[1], m[2])
	// Output: * **
}

func ExampleSeq_Partition() {
	// Both sides in one pass, each in encounter order.
	even, odd := catena.Range(1, 6, 1).Partition(func(n int) bool { return n%2 == 0 })
	fmt.Println(even, odd)
	// Output: [2 4] [1 3 5]
}

func ExampleAssociateWith() {
	m := catena.AssociateWith(catena.Of("go", "rust"), func(s string) int { return len(s) })
	fmt.Println(m["go"], m["rust"])
	// Output: 2 4
}

func ExampleTally() {
	fmt.Println(catena.Tally(catena.Of("a", "b", "a"))["a"])
	// Output: 2
}

func ExampleToKeySet() {
	set := catena.ToKeySet(catena.Of(1, 2, 2))
	_, has := set[2]
	fmt.Println(len(set), has)
	// Output: 2 true
}

// ---- aggregating ----

func ExampleSeq_MaxBy() {
	// Returns the ELEMENT with the largest key; ties go to the first.
	type run struct {
		Name string
		Secs int
	}
	runs := catena.Of(run{"ada", 42}, run{"bob", 51}, run{"eve", 51})
	slowest, _ := runs.MaxBy(func(r run) int { return r.Secs })
	fmt.Println(slowest.Name)
	// Output: bob
}

func ExampleSeq_MinBy() {
	type run struct {
		Name string
		Secs int
	}
	runs := catena.Of(run{"ada", 42}, run{"bob", 51})
	fastest, _ := runs.MinBy(func(r run) int { return r.Secs })
	fmt.Println(fastest.Name)
	// Output: ada
}

func ExampleSeq_MaxOf() {
	// Returns the KEY, where MaxBy returns the element — the -By/-Of
	// distinction, which holds across the whole library.
	longest, _ := catena.Of("go", "rust", "c").MaxOf(func(s string) int { return len(s) })
	fmt.Println(longest)
	// Output: 4
}

func ExampleSeq_MinOf() {
	shortest, _ := catena.Of("go", "rust", "c").MinOf(func(s string) int { return len(s) })
	fmt.Println(shortest)
	// Output: 1
}

func ExampleSeq_MinMaxOf() {
	// Both ends in a single pass.
	lo, hi, ok := catena.Of("go", "rust", "c").MinMaxOf(func(s string) int { return len(s) })
	fmt.Println(lo, hi, ok)
	// Output: 1 4 true
}

func ExampleSeq_MaxWith() {
	// A comparator, for orderings no single key expresses.
	v, _ := catena.Of("bb", "a", "cccc").
		MaxWith(func(x, y string) int { return len(x) - len(y) })
	fmt.Println(v)
	// Output: cccc
}

func ExampleSeq_MinWith() {
	v, _ := catena.Of("bb", "a", "cccc").
		MinWith(func(x, y string) int { return len(x) - len(y) })
	fmt.Println(v)
	// Output: a
}

func ExampleSeq_TopNBy() {
	// A bounded heap of n, not a sort: memory is O(n) rather than O(all),
	// which is the difference between a working pipeline and an OOM on a
	// large scan. Output is sorted descending, ties in encounter order.
	fmt.Println(catena.Of(5, 1, 9, 3, 7).TopNBy(3, catena.Self[int]))
	// Output: [9 7 5]
}

func ExampleSeq_BottomNBy() {
	fmt.Println(catena.Of(5, 1, 9, 3, 7).BottomNBy(2, catena.Self[int]))
	// Output: [1 3]
}

func ExampleSeq_SumOf() {
	type item struct{ Qty int }
	items := catena.Of(item{2}, item{3})
	fmt.Println(items.SumOf(func(i item) int { return i.Qty }))
	// Output: 5
}

func ExampleSeq_ProductOf() {
	// The empty product is 1, the multiplicative identity.
	fmt.Println(catena.Of(2, 3, 4).ProductOf(catena.Self[int]))
	fmt.Println(catena.Empty[int]().ProductOf(catena.Self[int]))
	// Output:
	// 24
	// 1
}

func ExampleSeq_AverageOf() {
	// Accumulates in float64, and reports false on empty rather than
	// dividing by zero.
	avg, ok := catena.Of(1, 2, 4).AverageOf(catena.Self[int])
	_, empty := catena.Empty[int]().AverageOf(catena.Self[int])
	fmt.Printf("%.2f %v %v\n", avg, ok, empty)
	// Output: 2.33 true false
}

func ExampleSeq_JoinToString() {
	type user struct{ Name string }
	users := catena.Of(user{"ada"}, user{"bob"})
	fmt.Println(users.JoinToString(", ", func(u user) string { return u.Name }))
	// Output: ada, bob
}

func ExampleMax() {
	v, ok := catena.Max(catena.Of(3, 9, 1))
	fmt.Println(v, ok)
	// Output: 9 true
}

func ExampleMin() {
	v, ok := catena.Min(catena.Of(3, 9, 1))
	fmt.Println(v, ok)
	// Output: 1 true
}

func ExampleMinMax() {
	lo, hi, ok := catena.MinMax(catena.Of(3, 9, 1))
	fmt.Println(lo, hi, ok)
	// Output: 1 9 true
}

func ExampleTopN() {
	fmt.Println(catena.TopN(catena.Of(5, 1, 9, 3), 2))
	// Output: [9 5]
}

func ExampleBottomN() {
	fmt.Println(catena.BottomN(catena.Of(5, 1, 9, 3), 2))
	// Output: [1 3]
}

func ExampleSum() {
	// Integer overflow wraps, exactly as + does.
	fmt.Println(catena.Sum(catena.Of(1, 2, 3)))
	// Output: 6
}

func ExampleProduct() {
	fmt.Println(catena.Product(catena.Of(2, 3, 4)))
	// Output: 24
}

func ExampleAverage() {
	avg, ok := catena.Average(catena.Of(2.0, 4.0))
	fmt.Println(avg, ok)
	// Output: 3 true
}

func ExampleJoin() {
	fmt.Println(catena.Join(catena.Of("a", "b", "c"), "-"))
	// Output: a-b-c
}
