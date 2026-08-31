package catena_test

// Cross-cutting contracts of the package itself: the argument table
// (construction panics carry the catena: prefix, zero counts are values),
// free interop with iter.Seq, and the version constant the release
// workflow verifies.

import (
	"iter"
	"slices"
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
