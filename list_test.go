package catena

// C15 — mirror consistency: for every generated List method,
// l.Op(args) ≡ l.AsSeq().Op(args) (collected). White-box so the test can
// read the generated manifest; the registry below must name every entry
// in generatedListMethods or TestListC15Completeness fails.

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"slices"
	"testing"
)

type c15Case struct {
	list func(List[int]) any
	seq  func(Seq[int]) any
}

type lpair struct{ A, B any }

func evenL(v int) bool { return v%2 == 0 }

var c15Inputs = [][]int{
	nil,
	{5},
	{1, 2, 3, 4, 5},
	{2, 2, 1, 3, 2, 2, 1},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
}

// ints normalizes a List result for comparison against a Seq Collect.
func ints(l List[int]) any { return []int(l) }

var c15Cases = map[string]c15Case{
	"Filter":        {func(l List[int]) any { return ints(l.Filter(evenL)) }, func(s Seq[int]) any { return s.Filter(evenL).Collect() }},
	"FilterNot":     {func(l List[int]) any { return ints(l.FilterNot(evenL)) }, func(s Seq[int]) any { return s.FilterNot(evenL).Collect() }},
	"FilterIndexed": {func(l List[int]) any { return ints(l.FilterIndexed(func(i, v int) bool { return i%2 == 0 })) }, func(s Seq[int]) any { return s.FilterIndexed(func(i, v int) bool { return i%2 == 0 }).Collect() }},
	"Take":          {func(l List[int]) any { return ints(l.Take(3)) }, func(s Seq[int]) any { return s.Take(3).Collect() }},
	"TakeWhile":     {func(l List[int]) any { return ints(l.TakeWhile(func(v int) bool { return v < 8 })) }, func(s Seq[int]) any { return s.TakeWhile(func(v int) bool { return v < 8 }).Collect() }},
	"TakeLast":      {func(l List[int]) any { return ints(l.TakeLast(3)) }, func(s Seq[int]) any { return s.TakeLast(3).Collect() }},
	"Drop":          {func(l List[int]) any { return ints(l.Drop(2)) }, func(s Seq[int]) any { return s.Drop(2).Collect() }},
	"DropWhile":     {func(l List[int]) any { return ints(l.DropWhile(func(v int) bool { return v < 5 })) }, func(s Seq[int]) any { return s.DropWhile(func(v int) bool { return v < 5 }).Collect() }},
	"DropLast":      {func(l List[int]) any { return ints(l.DropLast(3)) }, func(s Seq[int]) any { return s.DropLast(3).Collect() }},
	"Step":          {func(l List[int]) any { return ints(l.Step(3)) }, func(s Seq[int]) any { return s.Step(3).Collect() }},
	"OnEach": {
		func(l List[int]) any { sum := 0; r := ints(l.OnEach(func(v int) { sum += v })); return lpair{r, sum} },
		func(s Seq[int]) any {
			sum := 0
			r := s.OnEach(func(v int) { sum += v }).Collect()
			return lpair{r, sum}
		},
	},
	"Concat":      {func(l List[int]) any { return ints(l.Concat(Of(100, 101))) }, func(s Seq[int]) any { return s.Concat(Of(100, 101)).Collect() }},
	"Prepend":     {func(l List[int]) any { return ints(l.Prepend(100, 101)) }, func(s Seq[int]) any { return s.Prepend(100, 101).Collect() }},
	"Intersperse": {func(l List[int]) any { return ints(l.Intersperse(-9)) }, func(s Seq[int]) any { return s.Intersperse(-9).Collect() }},
	"IfEmpty":     {func(l List[int]) any { return ints(l.IfEmpty(42, 43)) }, func(s Seq[int]) any { return s.IfEmpty(42, 43).Collect() }},
	"WithIndex": {
		func(l List[int]) any { ks, vs := Unzip(l.WithIndex()); return lpair{ks, vs} },
		func(s Seq[int]) any { ks, vs := Unzip(s.WithIndex()); return lpair{ks, vs} },
	},
	"ZipWithNext": {
		func(l List[int]) any { ks, vs := Unzip(l.ZipWithNext()); return lpair{ks, vs} },
		func(s Seq[int]) any { ks, vs := Unzip(s.ZipWithNext()); return lpair{ks, vs} },
	},
	"SortedWith":   {func(l List[int]) any { return ints(l.SortedWith(func(a, b int) int { return b - a })) }, func(s Seq[int]) any { return s.SortedWith(func(a, b int) int { return b - a }).Collect() }},
	"DistinctWith": {func(l List[int]) any { return ints(l.DistinctWith(func(a, b int) bool { return a == b })) }, func(s Seq[int]) any { return s.DistinctWith(func(a, b int) bool { return a == b }).Collect() }},
	"Reversed":     {func(l List[int]) any { return ints(l.Reversed()) }, func(s Seq[int]) any { return s.Reversed().Collect() }},
	"Map":          {func(l List[int]) any { return ints(l.Map(func(v int) int { return v * 2 })) }, func(s Seq[int]) any { return s.Map(func(v int) int { return v * 2 }).Collect() }},
	"MapIndexed":   {func(l List[int]) any { return ints(l.MapIndexed(func(i, v int) int { return i*100 + v })) }, func(s Seq[int]) any { return s.MapIndexed(func(i, v int) int { return i*100 + v }).Collect() }},
	"FilterMap": {func(l List[int]) any { return ints(l.FilterMap(func(v int) (int, bool) { return v * 10, evenL(v) })) }, func(s Seq[int]) any {
		return s.FilterMap(func(v int) (int, bool) { return v * 10, evenL(v) }).Collect()
	}},
	"FlatMap":      {func(l List[int]) any { return ints(l.FlatMap(func(v int) Seq[int] { return Of(v, -v) })) }, func(s Seq[int]) any { return s.FlatMap(func(v int) Seq[int] { return Of(v, -v) }).Collect() }},
	"FlatMapSlice": {func(l List[int]) any { return ints(l.FlatMapSlice(func(v int) []int { return []int{v, -v} })) }, func(s Seq[int]) any { return s.FlatMapSlice(func(v int) []int { return []int{v, -v} }).Collect() }},
	"Scan":         {func(l List[int]) any { return ints(l.Scan(0, func(a, v int) int { return a + v })) }, func(s Seq[int]) any { return s.Scan(0, func(a, v int) int { return a + v }).Collect() }},
	"MapErr": {
		func(l List[int]) any {
			vs, errs := l.MapErr(func(v int) (int, error) { return v + 1, nil }).CollectAll()
			return lpair{vs, errs}
		},
		func(s Seq[int]) any {
			vs, errs := s.MapErr(func(v int) (int, error) { return v + 1, nil }).CollectAll()
			return lpair{vs, errs}
		},
	},
	"FilterErr": {
		func(l List[int]) any {
			vs, errs := l.FilterErr(func(v int) (bool, error) { return evenL(v), nil }).CollectAll()
			return lpair{vs, errs}
		},
		func(s Seq[int]) any {
			vs, errs := s.FilterErr(func(v int) (bool, error) { return evenL(v), nil }).CollectAll()
			return lpair{vs, errs}
		},
	},
	"DistinctBy":   {func(l List[int]) any { return ints(l.DistinctBy(func(v int) int { return v % 5 })) }, func(s Seq[int]) any { return s.DistinctBy(func(v int) int { return v % 5 }).Collect() }},
	"DedupeBy":     {func(l List[int]) any { return ints(l.DedupeBy(Self[int])) }, func(s Seq[int]) any { return s.DedupeBy(Self[int]).Collect() }},
	"SortedBy":     {func(l List[int]) any { return ints(l.SortedBy(func(v int) int { return v % 3 })) }, func(s Seq[int]) any { return s.SortedBy(func(v int) int { return v % 3 }).Collect() }},
	"SortedByDesc": {func(l List[int]) any { return ints(l.SortedByDesc(func(v int) int { return v % 3 })) }, func(s Seq[int]) any { return s.SortedByDesc(func(v int) int { return v % 3 }).Collect() }},
	"JoinBy": {
		func(l List[int]) any {
			return ints(l.JoinBy(Of(0, 1, 2), func(v int) int { return ((v % 3) + 3) % 3 }, Self[int], func(v, u int) int { return v*10 + u }))
		},
		func(s Seq[int]) any {
			return s.JoinBy(Of(0, 1, 2), func(v int) int { return ((v % 3) + 3) % 3 }, Self[int], func(v, u int) int { return v*10 + u }).Collect()
		},
	},
	"Collect": {func(l List[int]) any { return l.Collect() }, func(s Seq[int]) any { return s.Collect() }},
	"ToList":  {func(l List[int]) any { return l.ToList() }, func(s Seq[int]) any { return s.ToList() }},
	"ForEach": {
		func(l List[int]) any { sum := 0; l.ForEach(func(v int) { sum += v }); return sum },
		func(s Seq[int]) any { sum := 0; s.ForEach(func(v int) { sum += v }); return sum },
	},
	"ForEachIndexed": {
		func(l List[int]) any { sum := 0; l.ForEachIndexed(func(i, v int) { sum += i * v }); return sum },
		func(s Seq[int]) any { sum := 0; s.ForEachIndexed(func(i, v int) { sum += i * v }); return sum },
	},
	"ForEachErr": {
		func(l List[int]) any {
			return l.ForEachErr(func(v int) error {
				if v == 3 {
					return context.Canceled
				}
				return nil
			})
		},
		func(s Seq[int]) any {
			return s.ForEachErr(func(v int) error {
				if v == 3 {
					return context.Canceled
				}
				return nil
			})
		},
	},
	"Drain":     {func(l List[int]) any { l.Drain(); return nil }, func(s Seq[int]) any { s.Drain(); return nil }},
	"First":     {func(l List[int]) any { v, ok := l.First(); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.First(); return lpair{v, ok} }},
	"Last":      {func(l List[int]) any { v, ok := l.Last(); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.Last(); return lpair{v, ok} }},
	"Single":    {func(l List[int]) any { v, ok := l.Single(); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.Single(); return lpair{v, ok} }},
	"ElementAt": {func(l List[int]) any { v, ok := l.ElementAt(3); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.ElementAt(3); return lpair{v, ok} }},
	"Find":      {func(l List[int]) any { v, ok := l.Find(func(v int) bool { return v > 2 }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.Find(func(v int) bool { return v > 2 }); return lpair{v, ok} }},
	"FindLast":  {func(l List[int]) any { v, ok := l.FindLast(func(v int) bool { return v > 2 }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.FindLast(func(v int) bool { return v > 2 }); return lpair{v, ok} }},
	"FindIndex": {func(l List[int]) any { return l.FindIndex(func(v int) bool { return v > 2 }) }, func(s Seq[int]) any { return s.FindIndex(func(v int) bool { return v > 2 }) }},
	"FindMap": {
		func(l List[int]) any {
			v, ok := l.FindMap(func(v int) (int, bool) { return v * 10, v > 2 })
			return lpair{v, ok}
		},
		func(s Seq[int]) any {
			v, ok := s.FindMap(func(v int) (int, bool) { return v * 10, v > 2 })
			return lpair{v, ok}
		},
	},
	"Any":        {func(l List[int]) any { return l.Any(func(v int) bool { return v > 3 }) }, func(s Seq[int]) any { return s.Any(func(v int) bool { return v > 3 }) }},
	"All":        {func(l List[int]) any { return l.All(func(v int) bool { return v < 3 }) }, func(s Seq[int]) any { return s.All(func(v int) bool { return v < 3 }) }},
	"None":       {func(l List[int]) any { return l.None(func(v int) bool { return v > 3 }) }, func(s Seq[int]) any { return s.None(func(v int) bool { return v > 3 }) }},
	"Count":      {func(l List[int]) any { return l.Count() }, func(s Seq[int]) any { return s.Count() }},
	"CountWhere": {func(l List[int]) any { return l.CountWhere(evenL) }, func(s Seq[int]) any { return s.CountWhere(evenL) }},
	"IsEmpty":    {func(l List[int]) any { return l.IsEmpty() }, func(s Seq[int]) any { return s.IsEmpty() }},
	"Fold":       {func(l List[int]) any { return l.Fold(100, func(a, v int) int { return a + v }) }, func(s Seq[int]) any { return s.Fold(100, func(a, v int) int { return a + v }) }},
	"FoldIndexed": {
		func(l List[int]) any { return l.FoldIndexed(0, func(i, a, v int) int { return a + i*v }) },
		func(s Seq[int]) any { return s.FoldIndexed(0, func(i, a, v int) int { return a + i*v }) },
	},
	"FoldWhile": {
		func(l List[int]) any { return l.FoldWhile(0, func(a, v int) (int, bool) { a += v; return a, a < 6 }) },
		func(s Seq[int]) any { return s.FoldWhile(0, func(a, v int) (int, bool) { a += v; return a, a < 6 }) },
	},
	"FoldErr": {
		func(l List[int]) any {
			a, err := l.FoldErr(0, func(a, v int) (int, error) {
				if v == 3 {
					return 0, context.Canceled
				}
				return a + v, nil
			})
			return lpair{a, err}
		},
		func(s Seq[int]) any {
			a, err := s.FoldErr(0, func(a, v int) (int, error) {
				if v == 3 {
					return 0, context.Canceled
				}
				return a + v, nil
			})
			return lpair{a, err}
		},
	},
	"FoldBy": {
		func(l List[int]) any {
			return l.FoldBy(func(v int) int { return ((v % 3) + 3) % 3 }, func(k int) int { return k * 100 }, func(a, v int) int { return a + v })
		},
		func(s Seq[int]) any {
			return s.FoldBy(func(v int) int { return ((v % 3) + 3) % 3 }, func(k int) int { return k * 100 }, func(a, v int) int { return a + v })
		},
	},
	"Reduce":    {func(l List[int]) any { v, ok := l.Reduce(func(a, b int) int { return a + b }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.Reduce(func(a, b int) int { return a + b }); return lpair{v, ok} }},
	"GroupBy":   {func(l List[int]) any { return l.GroupBy(func(v int) int { return ((v % 3) + 3) % 3 }) }, func(s Seq[int]) any { return s.GroupBy(func(v int) int { return ((v % 3) + 3) % 3 }) }},
	"IndexBy":   {func(l List[int]) any { return l.IndexBy(func(v int) int { return ((v % 3) + 3) % 3 }) }, func(s Seq[int]) any { return s.IndexBy(func(v int) int { return ((v % 3) + 3) % 3 }) }},
	"TallyBy":   {func(l List[int]) any { return l.TallyBy(evenL) }, func(s Seq[int]) any { return s.TallyBy(evenL) }},
	"Associate": {func(l List[int]) any { return l.Associate(func(v int) (int, int) { return ((v % 3) + 3) % 3, v }) }, func(s Seq[int]) any { return s.Associate(func(v int) (int, int) { return ((v % 3) + 3) % 3, v }) }},
	"Partition": {
		func(l List[int]) any { yes, no := l.Partition(evenL); return lpair{yes, no} },
		func(s Seq[int]) any { yes, no := s.Partition(evenL); return lpair{yes, no} },
	},
	"MaxBy":     {func(l List[int]) any { v, ok := l.MaxBy(func(v int) int { return v % 3 }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.MaxBy(func(v int) int { return v % 3 }); return lpair{v, ok} }},
	"MinBy":     {func(l List[int]) any { v, ok := l.MinBy(func(v int) int { return v % 3 }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.MinBy(func(v int) int { return v % 3 }); return lpair{v, ok} }},
	"MaxOf":     {func(l List[int]) any { v, ok := l.MaxOf(func(v int) int { return -v }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.MaxOf(func(v int) int { return -v }); return lpair{v, ok} }},
	"MinOf":     {func(l List[int]) any { v, ok := l.MinOf(func(v int) int { return -v }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.MinOf(func(v int) int { return -v }); return lpair{v, ok} }},
	"MinMaxOf":  {func(l List[int]) any { a, b, ok := l.MinMaxOf(Self[int]); return [3]any{a, b, ok} }, func(s Seq[int]) any { a, b, ok := s.MinMaxOf(Self[int]); return [3]any{a, b, ok} }},
	"MaxWith":   {func(l List[int]) any { v, ok := l.MaxWith(func(a, b int) int { return a - b }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.MaxWith(func(a, b int) int { return a - b }); return lpair{v, ok} }},
	"MinWith":   {func(l List[int]) any { v, ok := l.MinWith(func(a, b int) int { return a - b }); return lpair{v, ok} }, func(s Seq[int]) any { v, ok := s.MinWith(func(a, b int) int { return a - b }); return lpair{v, ok} }},
	"TopNBy":    {func(l List[int]) any { return l.TopNBy(3, Self[int]) }, func(s Seq[int]) any { return s.TopNBy(3, Self[int]) }},
	"BottomNBy": {func(l List[int]) any { return l.BottomNBy(3, Self[int]) }, func(s Seq[int]) any { return s.BottomNBy(3, Self[int]) }},
	"SumOf":     {func(l List[int]) any { return l.SumOf(func(v int) int { return v * 2 }) }, func(s Seq[int]) any { return s.SumOf(func(v int) int { return v * 2 }) }},
	"ProductOf": {func(l List[int]) any { return l.ProductOf(func(v int) int { return v + 1 }) }, func(s Seq[int]) any { return s.ProductOf(func(v int) int { return v + 1 }) }},
	"AverageOf": {func(l List[int]) any { a, ok := l.AverageOf(Self[int]); return lpair{a, ok} }, func(s Seq[int]) any { a, ok := s.AverageOf(Self[int]); return lpair{a, ok} }},
	"JoinToString": {
		func(l List[int]) any {
			return l.JoinToString(",", func(v int) string { return string(rune('a' + ((v%26)+26)%26)) })
		},
		func(s Seq[int]) any {
			return s.JoinToString(",", func(v int) string { return string(rune('a' + ((v%26)+26)%26)) })
		},
	},
}

func TestListC15Mirror(t *testing.T) {
	for name, c := range c15Cases {
		t.Run(name, func(t *testing.T) {
			for _, in := range c15Inputs {
				got := c.list(List[int](in))
				want := c.seq(FromSlice(in))
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("input %v: List got %#v, Seq got %#v", in, got, want)
				}
			}
			// nil receiver ≡ empty list
			if got, want := c.list(nil), c.seq(nil); !reflect.DeepEqual(got, want) {
				t.Fatalf("nil: List got %#v, Seq got %#v", got, want)
			}
		})
	}
}

// TestListC15Completeness: every generated method must have a C15 case,
// every case must correspond to a generated method, and every hand-written
// list-only method must be named in listOnlyTested (backed by TestListOnly).
func TestListC15Completeness(t *testing.T) {
	gen := map[string]bool{}
	for _, name := range generatedListMethods {
		gen[name] = true
		if _, ok := c15Cases[name]; !ok {
			t.Errorf("generated List.%s has no C15 conformance case", name)
		}
	}
	for name := range c15Cases {
		if !gen[name] {
			t.Errorf("C15 case %q does not correspond to a generated method", name)
		}
	}

	listOnlyTested := map[string]bool{
		"Len": true, "At": true, "Get": true, "Slice": true, "Clone": true,
		"AsSeq": true, "FoldRight": true, "Append": true,
	}
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "list.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range f.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Recv != nil && d.Name.IsExported() {
			if !listOnlyTested[d.Name.Name] {
				t.Errorf("hand-written List.%s is not named in listOnlyTested (add a test in TestListOnly first)", d.Name.Name)
			}
		}
	}
}

func TestListOnly(t *testing.T) {
	l := List[int]{10, 20, 30}
	t.Run("Len_At_Get", func(t *testing.T) {
		if l.Len() != 3 || l.At(1) != 20 {
			t.Fatal("Len/At")
		}
		if v, ok := l.Get(2); v != 30 || !ok {
			t.Fatal("Get in range")
		}
		if _, ok := l.Get(-1); ok {
			t.Fatal("Get(-1)")
		}
		if _, ok := l.Get(3); ok {
			t.Fatal("Get out of range")
		}
		defer func() {
			if recover() == nil {
				t.Fatal("At out of range must panic like l[i]")
			}
		}()
		l.At(99)
	})
	t.Run("Slice_aliases", func(t *testing.T) {
		s := l.Slice(0, 2)
		s[0] = 99
		if l[0] != 99 {
			t.Fatal("Slice must share the backing array, like l[i:j]")
		}
		l[0] = 10
	})
	t.Run("Clone_is_fresh", func(t *testing.T) {
		c := l.Clone()
		c[0] = 99
		if l[0] == 99 {
			t.Fatal("Clone must not alias")
		}
		if got := List[int](nil).Clone(); got != nil {
			t.Fatal("Clone of nil is nil")
		}
	})
	t.Run("AsSeq_is_a_view", func(t *testing.T) {
		s := l.AsSeq()
		if got := s.Collect(); !slices.Equal(got, []int{10, 20, 30}) {
			t.Fatalf("got %v", got)
		}
		// multi-pass safe
		if got := s.Collect(); !slices.Equal(got, []int{10, 20, 30}) {
			t.Fatalf("second pass: %v", got)
		}
	})
	t.Run("FoldRight", func(t *testing.T) {
		got := List[string]{"a", "b", "c"}.FoldRight("|", func(v, acc string) string { return acc + v })
		if got != "|cba" {
			t.Fatalf("got %q", got)
		}
		if got := List[int](nil).FoldRight(7, func(v, a int) int { return a + v }); got != 7 {
			t.Fatalf("empty: %d", got)
		}
	})
	t.Run("Append_never_aliases", func(t *testing.T) {
		base := make(List[int], 2, 10) // spare capacity: built-in append WOULD alias
		base[0], base[1] = 1, 2
		out := base.Append(3)
		out[0] = 99
		if base[0] == 99 {
			t.Fatal("Append aliased the receiver's backing array")
		}
		if !slices.Equal([]int(out), []int{99, 2, 3}) {
			t.Fatalf("got %v", out)
		}
	})
}
