package par_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/par"
)

func ExampleMap() {
	// Eight lookups at once, results in the order the names went in.
	lookup := func(_ context.Context, name string) (int, error) {
		return len(name), nil
	}
	lengths, err := catena.Of("ada", "grace", "linus").
		Then(par.Map(context.Background(), 8, lookup)).
		Collect()
	fmt.Println(lengths, err)
	// Output: [3 5 5] <nil>
}

func ExampleMap_errors() {
	// A failed call is an element, so the consumer picks the policy:
	// CollectAll keeps going and gathers every error.
	parse := func(_ context.Context, s string) (int, error) {
		if s == "" {
			return 0, errors.New("empty")
		}
		return len(s), nil
	}
	lengths, errs := catena.Of("a", "", "ccc", "").
		Then(par.Map(context.Background(), 2, parse)).
		CollectAll()
	fmt.Println(lengths, errs)
	// Output: [1 3] [empty empty]
}

func ExampleMapUnordered() {
	// Results arrive as their calls finish, so the order is not the
	// input's; sort when it matters.
	upper := func(_ context.Context, s string) (string, error) {
		return strings.ToUpper(s), nil
	}
	got, _ := catena.Of("b", "c", "a").
		Then(par.MapUnordered(context.Background(), 3, upper)).
		Collect()
	slices.Sort(got)
	fmt.Println(got)
	// Output: [A B C]
}

func ExampleForEach() {
	// A concurrent loop that returns the first error, having cancelled
	// and waited for the rest.
	var total atomic.Int64
	add := func(_ context.Context, n int) error {
		total.Add(int64(n))
		return nil
	}
	err := catena.Range(1, 101, 1).Then(par.ForEach(context.Background(), 8, add))
	fmt.Println(total.Load(), err)
	// Output: 5050 <nil>
}

func ExampleBuffer() {
	// The source runs up to 16 elements ahead, on its own goroutine, so
	// a slow producer and a slow consumer overlap. Order is unchanged.
	squares := catena.Range(1, 6, 1).Map(func(n int) int { return n * n })
	fmt.Println(squares.Then(par.Buffer[int](16)).Collect())
	// Output: [1 4 9 16 25]
}

func ExampleChunkedWithin() {
	// Batches of up to three, or whatever has arrived within a minute.
	// This source never pauses, so every chunk closes on size.
	batches := catena.Range(1, 8, 1).
		Then(par.ChunkedWithin[int](3, time.Minute)).
		Collect()
	fmt.Println(batches)
	// Output: [[1 2 3] [4 5 6] [7]]
}

func ExampleMerge() {
	// Each sequence runs on its own goroutine; elements arrive as they
	// are produced, so only the order within one sequence is fixed.
	got := par.Merge(catena.Of(1, 2, 3), catena.Of(10, 20), catena.Of(100)).Collect()
	slices.Sort(got)
	fmt.Println(got)
	// Output: [1 2 3 10 20 100]
}
