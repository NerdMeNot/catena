package topn

import "github.com/NerdMeNot/catena"

type order struct{ amount int }

func amount(o order) int { return o.amount }

func flagged(xs catena.Seq[int], os catena.Seq[order], n int) {
	_ = catena.SortedDesc(xs).Take(10).Collect()     // want `SortedDesc followed by Take buffers and sorts the whole input to keep 10 elements: TopN keeps only that many`
	_ = catena.Sorted(xs).Take(n).Collect()          // want `Sorted followed by Take .* BottomN keeps`
	_ = os.SortedByDesc(amount).Take(3).Collect()    // want `SortedByDesc followed by Take .* TopNBy keeps`
	_ = os.SortedBy(amount).Take(n + 1).Collect()    // want `to keep n \+ 1 elements: BottomNBy keeps`
	_ = xs.Then(catena.SortedDesc).Take(2).Collect() // want `TopN keeps`
	_ = (catena.Sorted(xs)).Take(2).Collect()        // want `BottomN keeps`

	// Reported, but not fixed: the replacement returns a slice, which is
	// only a drop-in where the sequence was collected at once.
	for range catena.SortedDesc(xs).Take(3) { // want `TopN keeps`
	}
	_ = os.SortedByDesc(amount).Take(3).Map(amount) // want `TopNBy keeps`
	top := catena.Sorted(xs).Take(3)                // want `BottomN keeps`
	_ = top
}

func fine(xs catena.Seq[int], os catena.Seq[order]) {
	_ = catena.Sorted(xs).Collect()
	_ = xs.Take(3).Collect()
	_ = catena.Sorted(xs).Drop(1).Take(3).Collect()
	// No comparator form of TopN exists to suggest.
	_ = os.SortedWith(func(a, b order) int { return a.amount - b.amount }).Take(3).Collect()
	_ = catena.TopN(xs, 3)
	_ = catena.Distinct(xs).Take(3).Collect()
	_ = notCatena(xs).Take(3).Collect()
	// A method expression: its receiver is the first argument.
	_ = catena.Seq[int].Take(catena.Sorted(xs), 3)
}

func stillReported(xs catena.Seq[int], os catena.Seq[order]) {
	// Collect taken as a value rather than called: no fix to offer.
	collect := catena.Sorted(xs).Take(3).Collect // want `BottomN keeps`
	use(catena.SortedDesc(xs).Take(3).Collect)   // want `TopN keeps`
	_ = collect
	// Explicit type arguments are seen through.
	_ = os.SortedByDesc[int](amount).Take(3).Collect() // want `TopNBy keeps`
}

func use(func() []int) {}

func alsoFine(xs catena.Seq[int]) {
	_ = xs.FoldBy[int, int](func(v int) int { return v }, func(int) int { return 0 }, func(a, v int) int { return a + v })
}

func notCatena(xs catena.Seq[int]) catena.Seq[int] { return xs }
