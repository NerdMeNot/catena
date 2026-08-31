// Selection: finding extremes and top-k without sorting the world.
// TopNBy keeps a bounded heap of k — on a large scan that is the
// difference between kilobytes and the whole dataset in memory.
package main

import (
	"fmt"

	"github.com/NerdMeNot/catena"
)

type Player struct {
	Name  string
	Score int
	Level int
}

var players = []Player{
	{"ada", 972, 12}, {"bob", 512, 9}, {"eve", 972, 11}, {"kim", 730, 12},
	{"lou", 214, 3}, {"mia", 730, 8}, {"noa", 999, 14}, {"oli", 88, 2},
}

func main() {
	s := catena.FromSlice(players)

	// Top three by score. Output is sorted descending, and ties keep
	// their encounter order — ada scored 972 before eve, so ada ranks
	// ahead. That stability is guaranteed, not incidental.
	for i, p := range s.TopNBy(3, func(p Player) int { return p.Score }) {
		fmt.Printf("#%d %s (%d)\n", i+1, p.Name, p.Score)
	}

	// MaxBy returns the ELEMENT with the largest key; MaxOf the KEY
	// itself. The -By/-Of distinction holds across the whole library.
	best, _ := s.MaxBy(func(p Player) int { return p.Score })
	high, _ := s.MaxOf(func(p Player) int { return p.Score })
	fmt.Printf("best player: %s; high score: %d\n", best.Name, high)

	// One pass for both ends of the range.
	lo, hi, _ := s.MinMaxOf(func(p Player) int { return p.Level })
	fmt.Printf("levels span %d–%d\n", lo, hi)

	// SortedBy is stable and calls its selector exactly once per element,
	// so sorting by one key after another composes into a multi-level
	// sort: level descending, then score descending within a level.
	ranked := s.
		SortedByDesc(func(p Player) int { return p.Score }).
		SortedByDesc(func(p Player) int { return p.Level }).
		Take(4).
		Collect()
	fmt.Println("leaderboard by level, then score:")
	for _, p := range ranked {
		fmt.Printf("  L%-2d %-4s %d\n", p.Level, p.Name, p.Score)
	}

	// Reduce for custom pairwise selection when no key exists: the
	// player whose name sorts last, say.
	last, _ := s.Reduce(func(a, b Player) Player {
		if b.Name > a.Name {
			return b
		}
		return a
	})
	fmt.Println("alphabetically last:", last.Name)
}
