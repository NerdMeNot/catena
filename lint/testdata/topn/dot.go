package topn

import . "github.com/NerdMeNot/catena"

func dotted(xs Seq[int]) {
	_ = SortedDesc(xs).Take(3).Collect() // want `TopN keeps`
}
