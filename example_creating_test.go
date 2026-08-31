package catena_test

// Examples for the constructors and the slicing operators. See
// example_filtering_test.go for why these are Example functions.

import (
	"context"
	"fmt"
	"slices"

	"github.com/NerdMeNot/catena"
)

// ---- creating a sequence ----

func ExampleOf() {
	fmt.Println(catena.Of("go", "rust", "zig").Collect())
	// Output: [go rust zig]
}

func ExampleFromSlice() {
	// The slice is not copied: later mutations are visible to later
	// iterations, which is what makes this free.
	xs := []int{1, 2, 3}
	s := catena.FromSlice(xs)
	xs[0] = 99
	fmt.Println(s.Collect())
	// Output: [99 2 3]
}

func ExampleFrom() {
	// Takes the literal function type, so any iterator adapts without a
	// conversion at the call site — including the standard library's.
	fmt.Println(catena.From(slices.Values([]string{"a", "b"})).Collect())
	// Output: [a b]
}

func ExampleFromMap() {
	ages := map[string]int{"ada": 36}
	fmt.Println(catena.CollectMap(catena.FromMap(ages)))
	// Output: map[ada:36]
}

func ExampleFrom2() {
	pairs := catena.From2(func(yield func(string, int) bool) {
		yield("a", 1)
		yield("b", 2)
	})
	fmt.Println(pairs.MapTo(func(k string, v int) string {
		return fmt.Sprintf("%s=%d", k, v)
	}).Collect())
	// Output: [a=1 b=2]
}

func ExampleFromErrs() {
	rows := catena.FromErrs(func(yield func(int, error) bool) {
		yield(1, nil)
		yield(0, fmt.Errorf("row 2: corrupt"))
	})
	vals, err := rows.Collect()
	fmt.Println(vals, err)
	// Output: [1] row 2: corrupt
}

func ExampleFromChan() {
	ch := make(chan int, 3)
	for i := 1; i <= 3; i++ {
		ch <- i
	}
	close(ch)

	// Single-use, and it starts no goroutine: a sequence that is never
	// consumed never receives.
	fmt.Println(catena.FromChan(context.Background(), ch).Collect())
	// Output: [1 2 3]
}

func ExampleEmpty() {
	fmt.Println(catena.Empty[int]().Collect(), catena.Empty[int]().Count())
	// Output: [] 0
}

func ExampleEmpty2() {
	fmt.Println(catena.Empty2[string, int]().Count())
	// Output: 0
}

func ExampleEmptyTry() {
	vals, err := catena.EmptyTry[int]().Collect()
	fmt.Println(vals, err)
	// Output: [] <nil>
}

func ExampleOnce1() {
	// Once1, not Once: Once is the single-use guard method on Seq.
	fmt.Println(catena.Once1("only").Collect())
	// Output: [only]
}

func ExampleRepeat() {
	// Infinite, so it must be bounded by something downstream.
	fmt.Println(catena.Repeat("ha").Take(3).Collect())
	// Output: [ha ha ha]
}

func ExampleRepeatN() {
	fmt.Println(catena.RepeatN(0, 4).Collect())
	// Output: [0 0 0 0]
}

func ExampleGenerate() {
	// The seed is yielded first, then next applied repeatedly. Infinite.
	fmt.Println(catena.Generate(1, func(n int) int { return n * 3 }).
		Take(4).
		Collect())
	// Output: [1 3 9 27]
}

func ExampleGenerateWhile() {
	// The seed is yielded unconditionally; a value produced alongside
	// ok=false is not.
	fmt.Println(catena.GenerateWhile(1, func(n int) (int, bool) {
		return n * 3, n < 9
	}).Collect())
	// Output: [1 3 9]
}

func ExampleRange() {
	// Half-open, like a slice expression. A sign mismatch between step
	// and direction yields nothing rather than panicking, so a computed
	// step is safe.
	fmt.Println(catena.Range(0, 10, 3).Collect())
	fmt.Println(catena.Range(3, 0, -1).Collect())
	fmt.Println(catena.Range(0, 10, -1).Collect())
	// Output:
	// [0 3 6 9]
	// [3 2 1]
	// []
}

func ExampleCycle() {
	// Infinite — except over an empty source, which terminates rather
	// than spinning.
	fmt.Println(catena.Cycle(catena.Of("a", "b")).Take(5).Collect())
	fmt.Println(catena.Cycle(catena.Empty[string]()).Collect())
	// Output:
	// [a b a b a]
	// []
}

func ExampleSelf() {
	// The identity selector, for the -By operators when the element is
	// already the key.
	fmt.Println(catena.Of(3, 1, 2).TallyBy(catena.Self[int])[3])
	// Output: 1
}

// ---- slicing ----

func ExampleSeq_Take() {
	// Consumes exactly what it emits, so it bounds an infinite source.
	fmt.Println(catena.Generate(1, func(n int) int { return n + 1 }).
		Take(3).
		Collect())
	// Output: [1 2 3]
}

func ExampleSeq_TakeWhile() {
	// Stops at the first element that fails — unlike Filter, which would
	// keep testing the rest.
	fmt.Println(catena.Of(1, 2, 9, 3).
		TakeWhile(func(n int) bool { return n < 5 }).
		Collect())
	// Output: [1 2]
}

func ExampleSeq_TakeLast() {
	fmt.Println(catena.Range(1, 8, 1).TakeLast(3).Collect())
	// Output: [5 6 7]
}

func ExampleSeq_Drop() {
	fmt.Println(catena.Of("a", "b", "c", "d").Drop(2).Collect())
	// Output: [c d]
}

func ExampleSeq_DropWhile() {
	// Drops only the leading run; once the predicate fails, everything
	// after is kept.
	fmt.Println(catena.Of(0, 0, 3, 0, 5).
		DropWhile(func(n int) bool { return n == 0 }).
		Collect())
	// Output: [3 0 5]
}

func ExampleSeq_DropLast() {
	fmt.Println(catena.Range(1, 6, 1).DropLast(2).Collect())
	// Output: [1 2 3]
}

func ExampleSeq_Step() {
	// The first element always survives, then every nth after it.
	fmt.Println(catena.Range(0, 10, 1).Step(3).Collect())
	// Output: [0 3 6 9]
}
