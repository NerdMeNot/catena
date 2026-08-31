package catena_test

// §9.4: property-based equivalences, on top of the example-based
// conformance suite.

import (
	"maps"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
	"pgregory.net/rapid"
)

func genInts(t *rapid.T) []int {
	return rapid.SliceOfN(rapid.IntRange(-50, 50), 0, 40).Draw(t, "xs")
}

func TestPropFilterFusion(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		p := func(v int) bool { return v%2 == 0 }
		q := func(v int) bool { return v > 0 }
		a := catena.FromSlice(xs).Filter(p).Filter(q).Collect()
		b := catena.FromSlice(xs).Filter(func(v int) bool { return p(v) && q(v) }).Collect()
		if !slices.Equal(a, b) {
			t.Fatalf("%v vs %v", a, b)
		}
	})
}

func TestPropMapComposition(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		f := func(v int) int { return v * 3 }
		g := func(v int) int { return v - 7 }
		a := catena.FromSlice(xs).Map(f).Map(g).Collect()
		b := catena.FromSlice(xs).Map(func(v int) int { return g(f(v)) }).Collect()
		if !slices.Equal(a, b) {
			t.Fatalf("%v vs %v", a, b)
		}
	})
}

func TestPropTakeComposition(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		n := rapid.IntRange(0, 50).Draw(t, "n")
		m := rapid.IntRange(0, 50).Draw(t, "m")
		a := catena.FromSlice(xs).Take(n).Take(m).Collect()
		b := catena.FromSlice(xs).Take(min(n, m)).Collect()
		if !slices.Equal(a, b) {
			t.Fatalf("%v vs %v", a, b)
		}
	})
}

func TestPropDropTakeSlicing(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		n := rapid.IntRange(0, 45).Draw(t, "n")
		m := rapid.IntRange(0, 45).Draw(t, "m")
		got := catena.FromSlice(xs).Drop(n).Take(m).Collect()
		lo := min(n, len(xs))
		hi := min(lo+m, len(xs))
		want := xs[lo:hi]
		if !slices.Equal(got, slices.Clone(want)) && !(len(got) == 0 && len(want) == 0) {
			t.Fatalf("%v vs %v", got, want)
		}
	})
}

func TestPropCollectAgreement(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		s := catena.FromSlice(xs)
		a := s.Collect()
		b := []int(s.ToList())
		c := slices.Collect(s.Seq())
		if !slices.Equal(a, b) || !slices.Equal(b, c) {
			t.Fatalf("%v / %v / %v", a, b, c)
		}
	})
}

func TestPropDistinctEqualsSortedDedupe(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		asSet := func(vs []int) map[int]bool {
			m := map[int]bool{}
			for _, v := range vs {
				m[v] = true
			}
			return m
		}
		a := asSet(catena.Distinct(catena.FromSlice(xs)).Collect())
		b := asSet(catena.Dedupe(catena.Sorted(catena.FromSlice(xs))).Collect())
		if !maps.Equal(a, b) {
			t.Fatalf("%v vs %v", a, b)
		}
	})
}

func TestPropSortStability(t *testing.T) {
	type rec struct{ k, seq int }
	rapid.Check(t, func(t *rapid.T) {
		keys := rapid.SliceOfN(rapid.IntRange(0, 5), 0, 40).Draw(t, "keys")
		recs := make([]rec, len(keys))
		for i, k := range keys {
			recs[i] = rec{k, i}
		}
		sorted := catena.FromSlice(recs).SortedBy(func(r rec) int { return r.k }).Collect()
		for i := 1; i < len(sorted); i++ {
			if sorted[i-1].k == sorted[i].k && sorted[i-1].seq > sorted[i].seq {
				t.Fatalf("unstable at %d: %v", i, sorted)
			}
		}
	})
}

func TestPropTopNAgainstSort(t *testing.T) {
	type rec struct{ k, seq int }
	rapid.Check(t, func(t *rapid.T) {
		keys := rapid.SliceOfN(rapid.IntRange(0, 5), 0, 40).Draw(t, "keys")
		n := rapid.IntRange(0, 10).Draw(t, "n")
		recs := make([]rec, len(keys))
		for i, k := range keys {
			recs[i] = rec{k, i}
		}
		got := catena.FromSlice(recs).TopNBy(n, func(r rec) int { return r.k })
		want := catena.FromSlice(recs).SortedByDesc(func(r rec) int { return r.k }).Take(n).Collect()
		if !slices.Equal(got, want) {
			t.Fatalf("TopNBy %v vs SortedByDesc.Take %v", got, want)
		}
	})
}

func TestPropTryPartition(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		// build a Try where negatives are errors
		tr := catena.FromSlice(xs).MapErr(func(v int) (int, error) {
			if v < 0 {
				return 0, e1
			}
			return v, nil
		})
		vals, errs := tr.CollectAll()
		wantOK := 0
		for _, v := range xs {
			if v >= 0 {
				wantOK++
			}
		}
		if len(vals) != wantOK || len(errs) != len(xs)-wantOK {
			t.Fatalf("partition mismatch: %d/%d from %v", len(vals), len(errs), xs)
		}
		// Ignore ≡ successes of CollectAll; Errs count ≡ error count
		if !slices.Equal(tr.Ignore().Collect(), vals) {
			t.Fatal("Ignore disagrees with CollectAll")
		}
		if tr.Errs().Count() != len(errs) {
			t.Fatal("Errs count disagrees with CollectAll")
		}
	})
}

func TestPropSetOpsAgainstModel(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		a := rapid.SliceOfN(rapid.IntRange(0, 10), 0, 20).Draw(t, "a")
		b := rapid.SliceOfN(rapid.IntRange(0, 10), 0, 20).Draw(t, "b")
		inB := map[int]bool{}
		for _, v := range b {
			inB[v] = true
		}
		var wantI, wantE []int
		seen := map[int]bool{}
		for _, v := range a {
			if seen[v] {
				continue
			}
			seen[v] = true
			if inB[v] {
				wantI = append(wantI, v)
			} else {
				wantE = append(wantE, v)
			}
		}
		var wantU []int
		seen = map[int]bool{}
		for _, v := range append(slices.Clone(a), b...) {
			if !seen[v] {
				seen[v] = true
				wantU = append(wantU, v)
			}
		}
		sa, sb := catena.FromSlice(a), catena.FromSlice(b)
		if got := catena.Intersect(sa, sb).Collect(); !slices.Equal(got, wantI) {
			t.Fatalf("Intersect %v vs %v", got, wantI)
		}
		if got := catena.Except(sa, sb).Collect(); !slices.Equal(got, wantE) {
			t.Fatalf("Except %v vs %v", got, wantE)
		}
		if got := catena.Union(sa, sb).Collect(); !slices.Equal(got, wantU) {
			t.Fatalf("Union %v vs %v", got, wantU)
		}
	})
}

func TestPropChunkedRoundTrip(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		n := rapid.IntRange(1, 7).Draw(t, "n")
		got := catena.FlattenSlices(catena.Chunked(catena.FromSlice(xs), n)).Collect()
		if !slices.Equal(got, slices.Clone(xs)) && len(xs) > 0 {
			t.Fatalf("%v vs %v", got, xs)
		}
		// every chunk except the last has exactly n elements
		var sizes []int
		for c := range catena.Chunked(catena.FromSlice(xs), n).Seq() {
			sizes = append(sizes, len(c))
		}
		for i, s := range sizes {
			if i < len(sizes)-1 && s != n {
				t.Fatalf("chunk %d has %d elements, want %d", i, s, n)
			}
			if s == 0 || s > n {
				t.Fatalf("chunk %d has invalid size %d", i, s)
			}
		}
	})
}

func TestPropReversedInvolution(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		got := catena.FromSlice(xs).Reversed().Reversed().Collect()
		if !slices.Equal(got, slices.Clone(xs)) && len(xs) > 0 {
			t.Fatalf("%v vs %v", got, xs)
		}
	})
}

func TestPropFoldByEqualsGroupByFold(t *testing.T) {
	rapid.Check(t, func(t *rapid.T) {
		xs := genInts(t)
		key := func(v int) int { return ((v % 4) + 4) % 4 }
		a := catena.FromSlice(xs).FoldBy(key, func(int) int { return 0 }, func(acc, v int) int { return acc + v })
		b := map[int]int{}
		for k, bucket := range catena.FromSlice(xs).GroupBy(key) {
			sum := 0
			for _, v := range bucket {
				sum += v
			}
			b[k] = sum
		}
		if !maps.Equal(a, b) {
			t.Fatalf("%v vs %v", a, b)
		}
	})
}
