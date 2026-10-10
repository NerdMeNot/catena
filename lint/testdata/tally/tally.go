package tally

import (
	"fmt"

	"github.com/NerdMeNot/catena"
)

func initial(s string) byte { return s[0] }

func flagged(words catena.Seq[string]) {
	for k, bucket := range words.GroupBy(initial) { // want `GroupBy retains every element, and this loop only counts them: TallyBy keeps one count per key`
		fmt.Println(k, len(bucket))
	}
	total := 0
	for _, bucket := range words.ToList().GroupBy(initial) { // want `TallyBy keeps one count per key`
		if len(bucket) > 1 {
			total += len((bucket))
		}
	}
	_ = total
}

func fine(words catena.Seq[string]) {
	for k, bucket := range words.GroupBy(initial) {
		fmt.Println(k, len(bucket), bucket[0]) // the elements are used
	}
	for k := range words.GroupBy(initial) {
		fmt.Println(k)
	}
	for k, _ := range words.GroupBy(initial) {
		fmt.Println(k)
	}
	for range words.GroupBy(initial) {
	}
	var last []string
	for _, last = range words.GroupBy(initial) {
		fmt.Println(len(last))
	}
	_ = last // read after the loop
	groups := words.GroupBy(initial)
	for _, bucket := range groups {
		fmt.Println(len(bucket))
	}
	for _, bucket := range words.TallyBy(initial) {
		fmt.Println(bucket)
	}
	for _, bucket := range words.GroupBy(initial) {
		fmt.Println(cap(bucket), size(bucket), fmt.Sprint(bucket))
	}
	for _, v := range []string{"a"} {
		fmt.Println(len(v))
	}
}

func size(s []string) int { return len(s) }
