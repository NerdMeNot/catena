package minmax

import (
	"cmp"

	"github.com/NerdMeNot/catena"
)

type order struct{ amount int }

func amount(o order) int { return o.amount }

func byAmount(a, b order) int { return a.amount - b.amount }

func flagged(xs catena.Seq[float64], ns catena.Seq[int], ws catena.Seq[string], os catena.Seq[order]) {
	_, _ = catena.Sorted(xs).First()       // want `Sorted followed by First buffers and sorts the whole input to find one element: Min finds it in one pass`
	_, _ = catena.SortedDesc(xs).First()   // want `SortedDesc followed by First .* Max finds it`
	_, _ = os.SortedBy(amount).First()     // want `SortedBy followed by First .* MinBy finds it`
	_, _ = os.SortedByDesc(amount).First() // want `MaxBy finds it`
	_, _ = os.SortedWith(byAmount).First() // want `MinWith finds it`
	_, _ = xs.Then(catena.Sorted).First()  // want `Min finds it`

	// Last is the latest of several equal elements. For integers and
	// strings that is the same value as the earliest, so it has an
	// aggregate too.
	_, _ = catena.Sorted(ns).Last()               // want `Sorted followed by Last .* Max finds it`
	_, _ = catena.SortedDesc(ns).Last()           // want `SortedDesc followed by Last .* Min finds it`
	_, _ = ws.Then(catena.Sorted[string]).Last()  // want `Max finds it`
	_, _ = catena.Sorted(catena.Of(id(3))).Last() // want `Max finds it`
}

type id int

func fine(xs catena.Seq[float64], os catena.Seq[order]) {
	// For floats, equal is not identical: -0 == +0. Sorted(xs).Last() is
	// the latest of the equal maxima and Max(xs) the earliest, so the
	// rewrite would change the result.
	_, _ = catena.Sorted(xs).Last()
	_, _ = catena.SortedDesc(xs).Last()
	_, _ = xs.Then(catena.Sorted[float64]).Last()
	// A sequence written as a bare function has no element type to read.
	_, _ = catena.Sorted(func(yield func(int) bool) { yield(1) }).Last()
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

// In generic code the element type could be a float.
func generic[T cmp.Ordered](s catena.Seq[T]) {
	_, _ = catena.Sorted(s).Last()
	_, _ = catena.Sorted(s).First() // want `Min finds it`
}
