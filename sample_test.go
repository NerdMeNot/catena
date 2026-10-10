package catena_test

// Sample is random, so it cannot be registered against a model. What can
// be pinned down is: which elements it may return and in what order, that
// a seed makes it repeatable, that it draws from the right distribution,
// and that it does what makes it worth having — holds n elements and
// draws far fewer random numbers than it sees elements.

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

func seeded(seed uint64) *rand.Rand { return rand.New(rand.NewPCG(seed, seed^0x9e3779b97f4a7c15)) }

func TestSampleEdges(t *testing.T) {
	ten := catena.Range(0, 10, 1)
	if got := ten.Sample(0, seeded(1)); got != nil {
		t.Fatalf("n=0: %v", got)
	}
	if got := catena.Empty[int]().Sample(3, seeded(1)); got != nil {
		t.Fatalf("empty: %v", got)
	}
	var none catena.Seq[int]
	if got := none.Sample(3, nil); got != nil {
		t.Fatalf("nil sequence: %v", got)
	}
	// With n or fewer elements there is nothing to choose: all of them,
	// in order.
	for _, n := range []int{10, 11, 1000} {
		if got := ten.Sample(n, seeded(1)); !slices.Equal(got, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}) {
			t.Fatalf("n=%d: %v", n, got)
		}
	}
	if v := mustPanic(t, func() { ten.Sample(-1, nil) }); v != "catena: Sample: negative count" {
		t.Fatalf("panicked with %v", v)
	}
}

func TestSampleIsAnOrderedSubsetAndRepeatable(t *testing.T) {
	in := catena.Range(0, 1000, 1)
	for seed := range uint64(50) {
		got := in.Sample(20, seeded(seed))
		if len(got) != 20 || !slices.IsSorted(got) || len(slices.Compact(slices.Clone(got))) != 20 {
			t.Fatalf("seed %d: not 20 distinct elements in encounter order: %v", seed, got)
		}
		if again := in.Sample(20, seeded(seed)); !slices.Equal(got, again) {
			t.Fatalf("seed %d: %v then %v", seed, got, again)
		}
	}
	// The shared generator works too, and obeys the same shape.
	if got := in.Sample(20, nil); len(got) != 20 || !slices.IsSorted(got) {
		t.Fatalf("shared generator: %v", got)
	}
}

func TestSampleIsUniform(t *testing.T) {
	// Every 3-subset of 10 elements must be as likely as any other:
	// 120 subsets, 120,000 draws, so 1,000 each. A biased reservoir shows
	// up as subsets far from that; the bound is over six standard
	// deviations (about 31.5 each), so a correct one never trips it.
	const trials, want, tolerance = 120_000, 1000, 200
	in := catena.Range(0, 10, 1)
	rng := seeded(42)
	counts := map[string]int{}
	for range trials {
		counts[fmt.Sprint(in.Sample(3, rng))]++
	}
	if len(counts) != 120 {
		t.Fatalf("%d distinct subsets drawn, want all 120", len(counts))
	}
	for subset, n := range counts {
		if n < want-tolerance || n > want+tolerance {
			t.Errorf("subset %s drawn %d times, want about %d", subset, n, want)
		}
	}
}

// countingSource counts the random numbers drawn from it.
type countingSource struct {
	rand.Source
	draws int
}

func (c *countingSource) Uint64() uint64 { c.draws++; return c.Source.Uint64() }

func TestSampleDrawsFarFewerNumbersThanElements(t *testing.T) {
	const size, n = 1_000_000, 5
	src := &countingSource{Source: rand.NewPCG(7, 7)}
	consumed := 0
	got := counting(catena.Range(0, size, 1), &consumed).Sample(n, rand.New(src))
	if len(got) != n || !slices.IsSorted(got) || consumed != size {
		t.Fatalf("got %v after consuming %d", got, consumed)
	}
	// About 3·n·ln(size/n) draws — a couple of hundred — where deciding
	// per element would be a million.
	if src.draws > 2000 {
		t.Fatalf("%d random draws for %d elements", src.draws, size)
	}
}
