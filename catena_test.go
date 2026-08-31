package catena_test

// Cross-cutting contracts of the package itself: the argument table
// (construction panics carry the catena: prefix, zero counts are values),
// free interop with iter.Seq, and the version constant the release
// workflow verifies.

import (
	"iter"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/NerdMeNot/catena"
)

// ---- construction-time panics, all with the catena: prefix ----

func TestConstructionPanics(t *testing.T) {
	s := catena.Of(1, 2, 3)
	// Each case names the operator its panic must identify. Asserting only
	// the "catena: " prefix is not enough: it let BottomNBy, TopN and the
	// shared heap helper all report "TopNBy", which points a reader at a
	// function they never called.
	cases := []struct {
		name, op string
		call     func()
	}{
		{"Take", "Take", func() { s.Take(-1) }},
		{"Drop", "Drop", func() { s.Drop(-1) }},
		{"TakeLast", "TakeLast", func() { s.TakeLast(-1) }},
		{"DropLast", "DropLast", func() { s.DropLast(-1) }},
		{"Step/zero", "Step", func() { s.Step(0) }},
		{"Step/negative", "Step", func() { s.Step(-2) }},
		{"RepeatN", "RepeatN", func() { catena.RepeatN(1, -1) }},
		{"TopNBy", "TopNBy", func() { s.TopNBy(-1, catena.Self[int]) }},
		{"BottomNBy", "BottomNBy", func() { s.BottomNBy(-1, catena.Self[int]) }},
		{"TopN", "TopN", func() { catena.TopN(s, -1) }},
		{"BottomN", "BottomN", func() { catena.BottomN(s, -1) }},
		{"Chunked/zero", "Chunked", func() { catena.Chunked(s, 0) }},
		{"Chunked/negative", "Chunked", func() { catena.Chunked(s, -3) }},
		{"Windowed/size", "Windowed", func() { catena.Windowed(s, 0, 1) }},
		{"Windowed/step", "Windowed", func() { catena.Windowed(s, 1, 0) }},
		{"Range/zero step", "Range", func() { catena.Range(0, 10, 0) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := mustPanic(t, c.call)
			msg, ok := got.(string)
			if !ok {
				t.Fatalf("panicked with %#v, want a string message", got)
			}
			if want := "catena: " + c.op + ":"; !strings.HasPrefix(msg, want) {
				t.Fatalf("panicked with %q, want a message starting %q", msg, want)
			}
		})
	}
}

func TestZeroCountsAreValid(t *testing.T) {
	s := catena.Of(1, 2, 3)
	if got := s.Take(0).Collect(); got != nil {
		t.Fatalf("Take(0): %v", got)
	}
	if got := s.Drop(0).Collect(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("Drop(0): %v", got)
	}
	if got := s.TakeLast(0).Collect(); got != nil {
		t.Fatalf("TakeLast(0): %v", got)
	}
	if got := s.DropLast(0).Collect(); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("DropLast(0): %v", got)
	}
	if got := catena.RepeatN(9, 0).Collect(); got != nil {
		t.Fatalf("RepeatN 0: %v", got)
	}
	if got := s.TopNBy(0, catena.Self[int]); got != nil {
		t.Fatalf("TopNBy(0): %v", got)
	}
}

func TestSelf(t *testing.T) {
	if catena.Self(42) != 42 {
		t.Fatal("Self is not the identity")
	}
}

func TestInteropSeqConversion(t *testing.T) {
	s := catena.Of(1, 2, 3)
	var std iter.Seq[int] = s.Seq()
	got := slices.Collect(std)
	if !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("got %v", got)
	}
	// free conversions in both directions
	back := catena.From(std)
	if !catena.Equal(back, s) {
		t.Fatal("round trip failed")
	}
	if got := slices.Collect(iter.Seq[int](s)); !slices.Equal(got, []int{1, 2, 3}) {
		t.Fatalf("direct conversion: %v", got)
	}
	// Seq() on nil is a usable empty iterator
	if got := slices.Collect(catena.Seq[int](nil).Seq()); got != nil {
		t.Fatalf("nil Seq(): %v", got)
	}
}

func TestVersion(t *testing.T) {
	if !strings.HasPrefix(catena.Version, "0.") && !strings.HasPrefix(catena.Version, "1.") {
		t.Fatalf("Version %q does not look like a semantic version", catena.Version)
	}
}

// ---- the gathering terminals, across the block boundaries ----

// Collect and its relatives switch accumulation strategy at 1024 elements
// and again every 8192 after that. Statement coverage says nothing about
// whether the seams are right, so every terminal that gathers is checked
// either side of both, against a slice built the obvious way.
func TestGatheringAcrossBlockBoundaries(t *testing.T) {
	sizes := []int{0, 1, 1023, 1024, 1025, 9215, 9216, 9217, 17409}

	for _, n := range sizes {
		want := make([]int, n)
		for i := range want {
			want[i] = i
		}
		src := catena.FromSlice(want)

		t.Run("Collect/"+strconv.Itoa(n), func(t *testing.T) {
			got := src.Collect()
			if !slices.Equal(got, want) {
				t.Fatalf("len %d, want %d", len(got), n)
			}
			if n == 0 && got != nil {
				t.Fatal("empty must collect to nil")
			}
			// The whole point of the block strategy: no slack handed to
			// the caller to retain. Only above the switch — at or below
			// it the result is append's own slice, slack and all, which
			// is what it has always been.
			if n > 1024 && cap(got) != n {
				t.Fatalf("cap %d for %d elements — expected exact", cap(got), n)
			}
		})

		t.Run("ToList/"+strconv.Itoa(n), func(t *testing.T) {
			if got := src.ToList(); !slices.Equal([]int(got), want) {
				t.Fatalf("len %d, want %d", len(got), n)
			}
		})

		t.Run("Partition/"+strconv.Itoa(n), func(t *testing.T) {
			yes, no := src.Partition(func(v int) bool { return v%2 == 0 })
			if len(yes)+len(no) != n {
				t.Fatalf("%d + %d != %d", len(yes), len(no), n)
			}
			for _, v := range yes {
				if v%2 != 0 {
					t.Fatalf("odd %d on the yes side", v)
				}
			}
		})

		t.Run("Unzip/"+strconv.Itoa(n), func(t *testing.T) {
			ks, vs := catena.Unzip(src.WithIndex())
			if len(ks) != n || len(vs) != n {
				t.Fatalf("got %d/%d, want %d", len(ks), len(vs), n)
			}
			if !slices.Equal(vs, want) {
				t.Fatal("values lost their order")
			}
		})

		t.Run("TryCollect/"+strconv.Itoa(n), func(t *testing.T) {
			got, err := src.MapErr(func(v int) (int, error) { return v, nil }).Collect()
			if err != nil || !slices.Equal(got, want) {
				t.Fatalf("got %d elements, err %v", len(got), err)
			}
		})

		t.Run("CollectAll/"+strconv.Itoa(n), func(t *testing.T) {
			// every third element fails, so both gatherers cross the seam
			vals, errs := src.MapErr(func(v int) (int, error) {
				if v%3 == 2 {
					return 0, errBoom
				}
				return v, nil
			}).CollectAll()
			if len(vals)+len(errs) != n {
				t.Fatalf("%d + %d != %d", len(vals), len(errs), n)
			}
		})

		t.Run("SortedBy/"+strconv.Itoa(n), func(t *testing.T) {
			// reverse the input so the sort has real work, then check the
			// buffered entries survived the block seams in order
			rev := slices.Clone(want)
			slices.Reverse(rev)
			got := catena.FromSlice(rev).SortedBy(func(v int) int { return v }).Collect()
			if !slices.Equal(got, want) {
				t.Fatalf("sorted result wrong at n=%d", n)
			}
		})
	}
}

// Cycle replays from the same gathered buffer, so its first pass crossing
// a block seam must not disturb what it replays afterwards.
func TestCycleReplayAcrossBlocks(t *testing.T) {
	const n = 1500
	in := make([]int, n)
	for i := range in {
		in[i] = i
	}
	got := catena.Cycle(catena.FromSlice(in)).Take(n * 2).Collect()
	if len(got) != n*2 {
		t.Fatalf("got %d, want %d", len(got), n*2)
	}
	if !slices.Equal(got[:n], in) || !slices.Equal(got[n:], in) {
		t.Fatal("replayed pass differs from the first")
	}
}
