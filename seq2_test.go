package catena_test

// Dedicated Seq2 coverage: the bridge surface, nil receivers, statefulness
// of Take/Drop, and early termination for every operator.

import (
	"maps"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

type kv struct {
	K int
	V string
}

func pairs(items ...kv) catena.Seq2[int, string] {
	return func(yield func(int, string) bool) {
		for _, it := range items {
			if !yield(it.K, it.V) {
				return
			}
		}
	}
}

func collect2(s catena.Seq2[int, string]) []kv {
	var out []kv
	for k, v := range s.Seq2() {
		out = append(out, kv{k, v})
	}
	return out
}

func std() catena.Seq2[int, string] {
	return pairs(kv{1, "a"}, kv{2, "b"}, kv{3, "c"}, kv{4, "d"})
}

func TestSeq2Operators(t *testing.T) {
	t.Run("Filter", func(t *testing.T) {
		got := collect2(std().Filter(func(k int, v string) bool { return k%2 == 0 }))
		if !slices.Equal(got, []kv{{2, "b"}, {4, "d"}}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("FilterNot", func(t *testing.T) {
		got := collect2(std().FilterNot(func(k int, v string) bool { return k%2 == 0 }))
		if !slices.Equal(got, []kv{{1, "a"}, {3, "c"}}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Map", func(t *testing.T) {
		got := collect2(std().Map(func(k int, v string) (int, string) { return k * 10, v + "!" }))
		if !slices.Equal(got, []kv{{10, "a!"}, {20, "b!"}, {30, "c!"}, {40, "d!"}}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("MapValues", func(t *testing.T) {
		// f receives the key too (Kotlin-consistent)
		got := collect2(std().MapValues(func(k int, v string) string {
			if k == 2 {
				return "KEYED"
			}
			return v
		}))
		if !slices.Equal(got, []kv{{1, "a"}, {2, "KEYED"}, {3, "c"}, {4, "d"}}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("MapTo", func(t *testing.T) {
		got := std().MapTo(func(k int, v string) string { return v }).Collect()
		if !slices.Equal(got, []string{"a", "b", "c", "d"}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Take_Drop", func(t *testing.T) {
		if got := collect2(std().Take(2)); !slices.Equal(got, []kv{{1, "a"}, {2, "b"}}) {
			t.Fatalf("Take: %v", got)
		}
		if got := collect2(std().Drop(3)); !slices.Equal(got, []kv{{4, "d"}}) {
			t.Fatalf("Drop: %v", got)
		}
		if got := collect2(std().Take(0)); got != nil {
			t.Fatalf("Take(0): %v", got)
		}
		if got := collect2(std().Drop(0)); !slices.Equal(got, collect2(std())) {
			t.Fatalf("Drop(0): %v", got)
		}
		mustPanic(t, func() { std().Take(-1) })
		mustPanic(t, func() { std().Drop(-1) })
		// L1: re-iteration
		taken := std().Take(2)
		if a, b := collect2(taken), collect2(taken); !slices.Equal(a, b) {
			t.Fatalf("Take captured state: %v then %v", a, b)
		}
		dropped := std().Drop(2)
		if a, b := collect2(dropped), collect2(dropped); !slices.Equal(a, b) {
			t.Fatalf("Drop captured state: %v then %v", a, b)
		}
	})
	t.Run("Keys_Values_Swap", func(t *testing.T) {
		if got := std().Keys().Collect(); !slices.Equal(got, []int{1, 2, 3, 4}) {
			t.Fatalf("Keys: %v", got)
		}
		if got := std().Values().Collect(); !slices.Equal(got, []string{"a", "b", "c", "d"}) {
			t.Fatalf("Values: %v", got)
		}
		k, v, ok := std().Swap().First()
		if k != "a" || v != 1 || !ok {
			t.Fatalf("Swap: %v %v %v", k, v, ok)
		}
	})
	t.Run("Fold", func(t *testing.T) {
		got := std().Fold("", func(acc string, k int, v string) string { return acc + v })
		if got != "abcd" {
			t.Fatalf("got %q", got)
		}
	})
	t.Run("ForEach", func(t *testing.T) {
		sum := 0
		std().ForEach(func(k int, v string) { sum += k })
		if sum != 10 {
			t.Fatalf("got %d", sum)
		}
	})
	t.Run("Any_All", func(t *testing.T) {
		if !std().Any(func(k int, v string) bool { return v == "c" }) {
			t.Fatal("Any")
		}
		if std().Any(func(k int, v string) bool { return v == "z" }) {
			t.Fatal("Any false positive")
		}
		if !std().All(func(k int, v string) bool { return k < 10 }) {
			t.Fatal("All")
		}
		if std().All(func(k int, v string) bool { return k < 3 }) {
			t.Fatal("All false positive")
		}
	})
	t.Run("Count_First", func(t *testing.T) {
		if got := std().Count(); got != 4 {
			t.Fatalf("Count: %d", got)
		}
		k, v, ok := std().First()
		if k != 1 || v != "a" || !ok {
			t.Fatalf("First: %v %v %v", k, v, ok)
		}
		_, _, ok = catena.Empty2[int, string]().First()
		if ok {
			t.Fatal("First on empty")
		}
	})
	t.Run("Pull", func(t *testing.T) {
		next, stop := std().Pull()
		defer stop()
		k, v, ok := next()
		if k != 1 || v != "a" || !ok {
			t.Fatalf("got %v %v %v", k, v, ok)
		}
	})
	t.Run("FromMap_CollectMap_roundtrip", func(t *testing.T) {
		m := map[int]string{1: "a", 2: "b", 3: "c"}
		got := catena.CollectMap(catena.FromMap(m))
		if !maps.Equal(got, m) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestSeq2NilReceivers(t *testing.T) {
	var n catena.Seq2[int, string]
	if got := collect2(n.Filter(func(int, string) bool { return true })); got != nil {
		t.Fatal("Filter")
	}
	if got := n.Keys().Collect(); got != nil {
		t.Fatal("Keys")
	}
	if got := n.Values().Collect(); got != nil {
		t.Fatal("Values")
	}
	if got := collect2(n.Take(3)); got != nil {
		t.Fatal("Take")
	}
	if n.Count() != 0 {
		t.Fatal("Count")
	}
	if _, _, ok := n.First(); ok {
		t.Fatal("First")
	}
	if n.Any(func(int, string) bool { return true }) {
		t.Fatal("Any")
	}
	if !n.All(func(int, string) bool { return false }) {
		t.Fatal("All must be vacuously true")
	}
	next, stop := n.Pull()
	defer stop()
	if _, _, ok := next(); ok {
		t.Fatal("Pull on nil yielded")
	}
	ks, vs := catena.Unzip(n)
	if ks != nil || vs != nil {
		t.Fatal("Unzip")
	}
	if got := catena.CollectMap[int, string](nil); len(got) != 0 {
		t.Fatal("CollectMap")
	}
}

func TestSeq2EarlyTermination(t *testing.T) {
	inf := catena.From2(func(yield func(int, string) bool) {
		for i := 0; ; i++ {
			if !yield(i, "x") {
				return
			}
		}
	})
	seq2Ops := map[string]catena.Seq2[int, string]{
		"Filter":    inf.Filter(func(int, string) bool { return true }),
		"FilterNot": inf.FilterNot(func(int, string) bool { return false }),
		"Map":       inf.Map(func(k int, v string) (int, string) { return k, v }),
		"MapValues": inf.MapValues(func(k int, v string) string { return v }),
		"Take":      inf.Take(1000),
		"Drop":      inf.Drop(2),
	}
	for name, op := range seq2Ops {
		t.Run(name, func(t *testing.T) {
			mustComplete(t, testTimeout, func() {
				n := 0
				for range op.Seq2() {
					if n++; n == 3 {
						break
					}
				}
			})
		})
	}
	t.Run("Swap", func(t *testing.T) {
		mustComplete(t, testTimeout, func() {
			n := 0
			for range inf.Swap().Seq2() {
				if n++; n == 3 {
					break
				}
			}
		})
	})
	for name, op := range map[string]catena.Seq[string]{
		"MapTo":  inf.MapTo(func(k int, v string) string { return v }),
		"Values": inf.Values(),
	} {
		t.Run(name, func(t *testing.T) {
			mustComplete(t, testTimeout, func() { op.Take(3).Drain() })
		})
	}
	t.Run("Keys", func(t *testing.T) {
		mustComplete(t, testTimeout, func() { inf.Keys().Take(3).Drain() })
	})
}
