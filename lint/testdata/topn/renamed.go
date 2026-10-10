package topn

import c "github.com/NerdMeNot/catena"

func renamed(xs c.Seq[int]) {
	_ = c.SortedDesc(xs).Take(3).Collect() // want `TopN keeps`
}
