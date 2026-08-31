package catena_test

// Examples for Try, Seq2 and List. See example_filtering_test.go for why
// these are Example functions.

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/NerdMeNot/catena"
)

// parseAges is the fixture the Try examples share: two good values and one
// that fails, so every operator's treatment of an error is visible.
func parseAges() catena.Try[int] {
	return catena.Of("36", "unknown", "41").MapErr(strconv.Atoi)
}

// ---- Try: intermediates ----

func ExampleTry_Map() {
	// The callback never sees an errored element; it passes through
	// untouched, so a failure is not silently mapped into a valid value.
	doubled := parseAges().Map(func(n int) int { return n * 2 })
	vals, errs := doubled.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [72 82] 1
}

func ExampleTry_MapErr() {
	// A mapping that can itself fail. Errors from either stage flow on.
	tenths := parseAges().MapErr(func(n int) (int, error) {
		if n > 40 {
			return 0, errors.New("too old")
		}
		return n * 10, nil
	})
	vals, errs := tenths.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [360] 2
}

func ExampleTry_FlatMap() {
	// An errored input passes through un-mapped; inner errors flow in
	// order alongside the outer ones.
	pairs := parseAges().FlatMap(func(n int) catena.Try[int] {
		return catena.Of(n, n+1).MapErr(func(v int) (int, error) { return v, nil })
	})
	vals, errs := pairs.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [36 37 41 42] 1
}

func ExampleTry_Filter() {
	// The predicate is not called on errored elements, and they are not
	// filtered out — dropping them would silently discard failures.
	old := parseAges().Filter(func(n int) bool { return n > 40 })
	vals, errs := old.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [41] 1
}

func ExampleTry_FilterErr() {
	valid := parseAges().FilterErr(func(n int) (bool, error) {
		if n < 0 {
			return false, errors.New("negative")
		}
		return n > 40, nil
	})
	vals, errs := valid.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [41] 1
}

func ExampleTry_Take() {
	// Counts elements, errored or not — so it consumes at most n. For
	// "n successes", use Ignore().Take(n).
	first, _ := parseAges().Take(2).CollectAll()
	successes := parseAges().Ignore().Take(2).Collect()
	fmt.Println(first, successes)
	// Output: [36] [36 41]
}

func ExampleTry_TakeWhile() {
	// An errored element passes through without ending the sequence;
	// only a successful element failing the predicate stops it.
	kept, errs := parseAges().TakeWhile(func(n int) bool { return n < 40 }).CollectAll()
	fmt.Println(kept, len(errs))
	// Output: [36] 1
}

func ExampleTry_Drop() {
	rest, _ := parseAges().Drop(1).CollectAll()
	fmt.Println(rest)
	// Output: [41]
}

func ExampleTry_OnEach() {
	// Runs only on successes; errors pass by untouched.
	seen := 0
	parseAges().OnEach(func(int) { seen++ }).CollectAll()
	fmt.Println(seen)
	// Output: 2
}

func ExampleTry_OnError() {
	// The logging tap, the mirror of OnEach.
	logged := 0
	parseAges().OnError(func(error) { logged++ }).CollectAll()
	fmt.Println(logged)
	// Output: 1
}

func ExampleTry_Recover() {
	// Repair chosen errors mid-stream: reporting true replaces the
	// element, false lets the error continue.
	fixed := parseAges().Recover(func(err error) (int, bool) {
		return 0, strings.Contains(err.Error(), "unknown")
	})
	vals, errs := fixed.CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [36 0 41] 0
}

func ExampleTry_WrapErr() {
	// Add the context that only this stage has. Returning nil keeps the
	// original error rather than turning a failure into a zero value.
	wrapped := parseAges().WrapErr(func(err error) error {
		return fmt.Errorf("parsing ages: %w", err)
	})
	_, err := wrapped.Collect()
	fmt.Println(strings.HasPrefix(err.Error(), "parsing ages:"))
	// Output: true
}

func ExampleTry_UntilDone() {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := parseAges().UntilDone(ctx).Collect()
	fmt.Println(err)
	// Output: context canceled
}

// ---- Try: terminals, one per error policy ----

func ExampleTry_Collect() {
	// Abort: the values gathered before the failure, plus the error.
	vals, err := parseAges().Collect()
	fmt.Println(vals, err != nil)
	// Output: [36] true
}

func ExampleTry_CollectAll() {
	// Gather: everything that worked and everything that did not, in one
	// pass. The two slices do not correspond positionally.
	vals, errs := parseAges().CollectAll()
	fmt.Println(vals, len(errs))
	// Output: [36 41] 1
}

func ExampleTry_Ignore() {
	// Skip: drop the failures and carry on as a plain Seq.
	fmt.Println(parseAges().Ignore().Collect())
	// Output: [36 41]
}

func ExampleTry_Errs() {
	// The dual of Ignore. Consuming both on one single-pass Try is a
	// double consume — use CollectAll instead.
	fmt.Println(parseAges().Errs().Count())
	// Output: 1
}

func ExampleTry_Fold() {
	// Stops at the first error, returning the accumulator so far.
	sum, err := parseAges().Fold(0, func(acc, n int) int { return acc + n })
	fmt.Println(sum, err != nil)
	// Output: 36 true
}

func ExampleTry_ForEach() {
	// Stops at the first of an element error or a callback error.
	err := parseAges().ForEach(func(n int) error {
		fmt.Println("handled", n)
		return nil
	})
	fmt.Println(err != nil)
	// Output:
	// handled 36
	// true
}

func ExampleTry_Err() {
	// Just the first error, if any — for pipelines run entirely for
	// their side effects.
	fmt.Println(parseAges().Err() != nil)
	// Output: true
}

func ExampleTry_Count() {
	// Successes counted up to the first error, which is returned with it.
	n, err := parseAges().Count()
	fmt.Println(n, err != nil)
	// Output: 1 true
}

func ExampleTry_Must() {
	// For pipelines where a failure is a programming bug. The panic
	// value is the error itself, so recover() can inspect it.
	defer func() { fmt.Println("recovered:", recover()) }()
	parseAges().Must().Drain()
	// Output: recovered: strconv.Atoi: parsing "unknown": invalid syntax
}

func ExampleTry_Pull() {
	next, stop := parseAges().Pull()
	defer stop()
	v, err, ok := next()
	fmt.Println(v, err, ok)
	// Output: 36 <nil> true
}

func ExampleTry_Seq2() {
	// A free conversion to the standard pair iterator.
	for v, err := range parseAges().Seq2() {
		if err != nil {
			fmt.Println("error at", v)
			break
		}
		fmt.Println("ok", v)
	}
	// Output:
	// ok 36
	// error at 0
}

// ---- Seq2: the bridge back to Seq ----

func ExampleSeq2_Filter() {
	pairs := catena.Of(1, 2, 3, 4).WithIndex().
		Filter(func(i, v int) bool { return v%2 == 0 })
	fmt.Println(pairs.Values().Collect())
	// Output: [2 4]
}

func ExampleSeq2_FilterNot() {
	pairs := catena.Of(1, 2, 3).WithIndex().
		FilterNot(func(i, v int) bool { return v == 2 })
	fmt.Println(pairs.Values().Collect())
	// Output: [1 3]
}

func ExampleSeq2_Map() {
	pairs := catena.Of("a", "b").WithIndex().
		Map(func(i int, s string) (string, int) { return s, i * 10 })
	fmt.Println(catena.CollectMap(pairs))
	// Output: map[a:0 b:10]
}

func ExampleSeq2_MapValues() {
	// The callback receives the key as well as the value.
	pairs := catena.Of("a", "b").WithIndex().
		MapValues(func(i int, s string) string { return fmt.Sprintf("%d%s", i, s) })
	fmt.Println(pairs.Values().Collect())
	// Output: [0a 1b]
}

func ExampleSeq2_MapTo() {
	// The intended exit: collapse each pair into one value and continue
	// in Seq, where the full API lives.
	fmt.Println(catena.Of("a", "b").WithIndex().
		MapTo(func(i int, s string) string { return fmt.Sprintf("%d%s", i, s) }).
		Collect())
	// Output: [0a 1b]
}

func ExampleSeq2_Take() {
	fmt.Println(catena.Range(1, 9, 1).WithIndex().Take(3).Values().Collect())
	// Output: [1 2 3]
}

func ExampleSeq2_Drop() {
	fmt.Println(catena.Range(1, 5, 1).WithIndex().Drop(2).Values().Collect())
	// Output: [3 4]
}

func ExampleSeq2_Keys() {
	// Keys and Values on the SAME single-pass Seq2 is a double consume;
	// Unzip does both in one pass.
	fmt.Println(catena.Of("a", "b").WithIndex().Keys().Collect())
	// Output: [0 1]
}

func ExampleSeq2_Values() {
	fmt.Println(catena.Of("a", "b").WithIndex().Values().Collect())
	// Output: [a b]
}

func ExampleSeq2_Swap() {
	pairs := catena.Of("a", "b").WithIndex().Swap()
	fmt.Println(pairs.Keys().Collect())
	// Output: [a b]
}

func ExampleSeq2_Fold() {
	total := catena.Of(10, 20, 30).WithIndex().
		Fold(0, func(acc, i, v int) int { return acc + i*v })
	fmt.Println(total)
	// Output: 80
}

func ExampleSeq2_ForEach() {
	catena.Of("a", "b").WithIndex().ForEach(func(i int, s string) {
		fmt.Printf("%d=%s ", i, s)
	})
	// Output: 0=a 1=b
}

func ExampleSeq2_Any() {
	fmt.Println(catena.Of(1, 2).WithIndex().Any(func(i, v int) bool { return v == 2 }))
	// Output: true
}

func ExampleSeq2_All() {
	fmt.Println(catena.Of(2, 4).WithIndex().All(func(i, v int) bool { return v%2 == 0 }))
	// Output: true
}

func ExampleSeq2_Count() {
	fmt.Println(catena.Of("a", "b", "c").WithIndex().Count())
	// Output: 3
}

func ExampleSeq2_First() {
	i, v, ok := catena.Of("a", "b").WithIndex().First()
	fmt.Println(i, v, ok)
	// Output: 0 a true
}

func ExampleSeq2_Pull() {
	next, stop := catena.Of("a", "b").WithIndex().Pull()
	defer stop()
	i, v, ok := next()
	fmt.Println(i, v, ok)
	// Output: 0 a true
}

func ExampleSeq2_Seq2() {
	for i, v := range catena.Of("a", "b").WithIndex().Seq2() {
		fmt.Printf("%d=%s ", i, v)
	}
	// Output: 0=a 1=b
}

func ExampleCollectMap() {
	// To a map. On a duplicate key the last value wins, as with plain
	// map assignment.
	fmt.Println(catena.CollectMap(catena.Of("a", "b").WithIndex()))
	// Output: map[0:a 1:b]
}

func ExampleUnzip() {
	// Both sides in one pass — the safe way to get keys and values from
	// a single-use Seq2.
	idx, vals := catena.Unzip(catena.Of("a", "b").WithIndex())
	fmt.Println(idx, vals)
	// Output: [0 1] [a b]
}

// ---- List: the eager mirror ----

func ExampleList_Len() {
	l := catena.List[int]{3, 1, 2}
	fmt.Println(l.Len())
	// Output: 3
}

func ExampleList_At() {
	// Panics exactly like l[i] — it IS the index expression.
	l := catena.List[string]{"a", "b"}
	fmt.Println(l.At(1))
	// Output: b
}

func ExampleList_Get() {
	// The comma-ok form, for when the index may be out of range.
	l := catena.List[string]{"a"}
	v, ok := l.Get(0)
	_, bad := l.Get(9)
	fmt.Println(v, ok, bad)
	// Output: a true false
}

func ExampleList_Slice() {
	// It is the slice expression, aliasing included.
	l := catena.List[int]{1, 2, 3, 4}
	fmt.Println(l.Slice(1, 3))
	// Output: [2 3]
}

func ExampleList_Clone() {
	l := catena.List[int]{1, 2}
	c := l.Clone()
	c[0] = 99
	fmt.Println(l, c)
	// Output: [1 2] [99 2]
}

func ExampleList_AsSeq() {
	// Crossing to lazy is explicit, and free — it is a view, not a copy.
	l := catena.List[int]{1, 2, 3, 4}
	first, _ := l.AsSeq().Find(func(n int) bool { return n > 2 })
	fmt.Println(first)
	// Output: 3
}

func ExampleList_FoldRight() {
	// Exists only on List: a right fold needs the whole sequence, which
	// a List already is.
	l := catena.List[string]{"a", "b", "c"}
	fmt.Println(l.FoldRight("|", func(s, acc string) string { return "(" + s + acc + ")" }))
	// Output: (a(b(c|)))
}

func ExampleList_Append() {
	// Unlike the builtin, the result never aliases the receiver — every
	// List transform returns fresh backing memory.
	base := make(catena.List[int], 2, 10)
	grown := base.Append(9)
	grown[0] = 99
	fmt.Println(base, grown)
	// Output: [0 0] [99 0 9]
}
