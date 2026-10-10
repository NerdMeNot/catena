package catena_test

// Behaviour tests for the operators that keep a chain going where it used
// to stop: Then, FoldedBy, GroupedBy, the outer joins and the sorted
// merge. The conformance registrations prove these agree with a model on
// the standard inputs; what is here are the claims their doc comments
// make that a model comparison does not reach — when a second sequence is
// read, how often a callback runs, what order keys come out in.

import (
	"fmt"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

func TestJoinsReadTheRightSideOncePerIteration(t *testing.T) {
	key := catena.Self[int]
	joins := map[string]func(left, right catena.Seq[int]) catena.Seq[int]{
		"JoinBy": func(l, r catena.Seq[int]) catena.Seq[int] {
			return l.JoinBy(r, key, key, func(v, u int) int { return v + u })
		},
		"LeftJoinBy": func(l, r catena.Seq[int]) catena.Seq[int] {
			return l.LeftJoinBy(r, key, key, func(v, u int, _ bool) int { return v + u })
		},
		"GroupJoinBy": func(l, r catena.Seq[int]) catena.Seq[int] {
			return l.GroupJoinBy(r, key, key, func(v int, us []int) int { return v + len(us) })
		},
	}
	for name, join := range joins {
		t.Run(name, func(t *testing.T) {
			var starts, cleanups int
			joined := join(infinite(), tracked(5, &starts, &cleanups))
			if starts != 0 {
				t.Fatal("building the join read the right side")
			}
			// An early stop on the left still leaves the right side
			// read exactly once, to its end, and cleaned up.
			joined.Take(2).Drain()
			if starts != 1 || cleanups != 1 {
				t.Fatalf("after one iteration: starts=%d cleanups=%d", starts, cleanups)
			}
			// Nothing is cached between iterations.
			joined.Take(2).Drain()
			if starts != 2 || cleanups != 2 {
				t.Fatalf("after two iterations: starts=%d cleanups=%d", starts, cleanups)
			}
			// The right side is buffered before the left is touched,
			// so it is read even when the left is empty.
			join(nil, tracked(5, &starts, &cleanups)).Drain()
			if starts != 3 {
				t.Fatalf("empty left: starts=%d", starts)
			}
		})
	}
}

func TestOuterJoinsWithNoRightSide(t *testing.T) {
	key := catena.Self[int]
	left := catena.Of(1, 2)
	got := left.LeftJoinBy(nil, key, key, func(v, u int, ok bool) string { return fmt.Sprint(v, u, ok) }).Collect()
	if !slices.Equal(got, []string{"1 0 false", "2 0 false"}) {
		t.Fatalf("LeftJoinBy: %v", got)
	}
	groups := left.GroupJoinBy(catena.Empty[int](), key, key, func(v int, us []int) string { return fmt.Sprint(v, us == nil) }).Collect()
	if !slices.Equal(groups, []string{"1 true", "2 true"}) {
		t.Fatalf("GroupJoinBy: %v", groups)
	}
}

func TestLeftJoinByOrderAndMultiplicity(t *testing.T) {
	// Left encounter order, then right encounter order within a key; a
	// duplicated left key repeats its matches; an unmatched left element
	// appears exactly once.
	tens := func(u int) int { return u / 10 }
	got := catena.Of(2, 9, 1, 2).LeftJoinBy(catena.Of(21, 10, 20, 11), catena.Self[int], tens,
		func(v, u int, ok bool) string {
			if !ok {
				return fmt.Sprint(v, ":none")
			}
			return fmt.Sprint(v, ":", u)
		}).Collect()
	want := []string{"2:21", "2:20", "9:none", "1:10", "1:11", "2:21", "2:20"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestFoldedByCallsInitOncePerKeyAndIsLazy(t *testing.T) {
	var consumed int
	var inits []string
	folded := counting(catena.Of(3, 1, 4, 1, 5, 9, 2, 6), &consumed).FoldedBy(
		func(v int) string { return []string{"even", "odd"}[v%2] },
		func(k string) int { inits = append(inits, k); return 0 },
		func(a, v int) int { return a + v },
	)
	if consumed != 0 || inits != nil {
		t.Fatal("building FoldedBy consumed the source")
	}
	keys, sums := catena.Unzip(folded)
	// Keys in first-seen order: 3 is odd, so odd comes first.
	if !slices.Equal(keys, []string{"odd", "even"}) || !slices.Equal(sums, []int{19, 12}) {
		t.Fatalf("keys=%v sums=%v", keys, sums)
	}
	if !slices.Equal(inits, []string{"odd", "even"}) {
		t.Fatalf("init called for %v, want once per key in first-seen order", inits)
	}
	// Stopping after the first pair is honoured.
	if k, _, ok := folded.First(); !ok || k != "odd" {
		t.Fatalf("First = %v, %v", k, ok)
	}
}

func TestGroupedByKeepsFirstSeenOrder(t *testing.T) {
	var got []string
	for k, vs := range catena.GroupedBy(catena.Of("pear", "apple", "plum", "fig", "avocado"), func(w string) byte { return w[0] }) {
		got = append(got, fmt.Sprintf("%c=%v", k, vs))
	}
	want := []string{"p=[pear plum]", "a=[apple avocado]", "f=[fig]"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if catena.GroupedBy(catena.Empty[int](), catena.Self[int]).Count() != 0 {
		t.Fatal("an empty source yielded a group")
	}
}

func TestThenOnNilAndAsATerminal(t *testing.T) {
	var s catena.Seq[int]
	// A nil sequence is passed through as it is: Then adds nothing.
	if got := s.Then(func(s catena.Seq[int]) bool { return s == nil }); !got {
		t.Fatal("Then did not pass the nil receiver through")
	}
	if got := s.Then(catena.Sum); got != 0 {
		t.Fatalf("Sum over nil = %d", got)
	}
	// U may be anything, including a function or a pair of sequences
	// packed in a struct.
	type halves struct{ evens, odds []int }
	split := func(s catena.Seq[int]) halves {
		e, o := s.Partition(even)
		return halves{e, o}
	}
	if got := catena.Of(1, 2, 3, 4).Then(split); !slices.Equal(got.evens, []int{2, 4}) || !slices.Equal(got.odds, []int{1, 3}) {
		t.Fatalf("got %+v", got)
	}
}
