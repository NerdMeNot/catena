package unusedseq

import (
	"fmt"
	"strconv"

	"github.com/NerdMeNot/catena"
)

func double(n int) int { return n * 2 }

func show(n int) { fmt.Println(n) }

func build() catena.Seq[int] { return catena.Of(1) }

func flagged(xs catena.Seq[int], ss catena.Seq[string]) {
	xs.OnEach(show)          // want `this Seq is built and discarded: a lazy sequence runs nothing until a terminal \(Collect, ForEach, Drain, …\) consumes it`
	xs.Map(double)           // want `this Seq is built and discarded`
	catena.Sorted(xs)        // want `this Seq is built and discarded`
	xs.WithIndex()           // want `this Seq2 is built and discarded`
	ss.MapErr(strconv.Atoi)  // want `this Try is built and discarded`
	build()                  // want `this Seq is built and discarded`
	(xs.Take(1))             // want `this Seq is built and discarded`
	xs.Then(catena.Distinct) // want `this Seq is built and discarded`
}

func fine(xs catena.Seq[int]) {
	xs.Drain()
	xs.ForEach(show)
	xs.OnEach(show).Drain()
	_ = xs.Map(double) // discarded on purpose
	ys := xs.Map(double)
	ys.Collect() // a slice: unused, but not lazy
	xs.ToList()
	xs.Then(catena.Sum)
	fmt.Println()
	func() {}()
	ch := make(chan int, 1)
	ch <- 1
	<-ch // an expression statement that is not a call
}
