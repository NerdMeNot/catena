package minmax

import "github.com/NerdMeNot/catena"

type order struct{ amount int }

func amount(o order) int { return o.amount }

func byAmount(a, b order) int { return a.amount - b.amount }

func flagged(xs catena.Seq[float64], os catena.Seq[order]) {
	_, _ = catena.Sorted(xs).First()              // want `Sorted followed by First buffers and sorts the whole input to find one element: Min finds it in one pass`
	_, _ = catena.Sorted(xs).Last()               // want `Sorted followed by Last .* Max finds it`
	_, _ = catena.SortedDesc(xs).First()          // want `SortedDesc followed by First .* Max finds it`
	_, _ = catena.SortedDesc(xs).Last()           // want `SortedDesc followed by Last .* Min finds it`
	_, _ = os.SortedBy(amount).First()            // want `SortedBy followed by First .* MinBy finds it`
	_, _ = os.SortedByDesc(amount).First()        // want `MaxBy finds it`
	_, _ = os.SortedWith(byAmount).First()        // want `MinWith finds it`
	_, _ = xs.Then(catena.Sorted).First()         // want `Min finds it`
	_, _ = xs.Then(catena.Sorted[float64]).Last() // want `Max finds it`
}

func fine(xs catena.Seq[float64], os catena.Seq[order]) {
	// A stable sort puts the LATEST of several equal keys last; MaxBy
	// returns the earliest. Not the same element, so not reported.
	_, _ = os.SortedBy(amount).Last()
	_, _ = os.SortedByDesc(amount).Last()
	_, _ = os.SortedWith(byAmount).Last()
	_, _ = xs.First()
	_, _ = xs.Filter(func(float64) bool { return true }).First()
	_, _ = catena.Distinct(xs).Last()
	_, _ = xs.Then(func(s catena.Seq[float64]) catena.Seq[float64] { return s }).First()
	_, _ = catena.Min(xs)
}
