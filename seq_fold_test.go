package catena_test

// Conformance registrations for seq_fold.go. The models are the hand-
// written loops the operators must agree with; the collision and ordering
// policies (last-wins maps, encounter-order buckets) are encoded in them.

import (
	"github.com/NerdMeNot/catena"
)

func init() {
	registerTermOps([]termOp{
		{
			name:   "Fold",
			covers: []string{"Fold"},
			op:     func(s catena.Seq[int]) any { return s.Fold(100, func(a, v int) int { return a + v }) },
			model: func(in []int) any {
				acc := 100
				for _, v := range in {
					acc += v
				}
				return acc
			},
		},
		{
			name:   "FoldIndexed",
			covers: []string{"FoldIndexed"},
			op: func(s catena.Seq[int]) any {
				return s.FoldIndexed(0, func(i, a, v int) int { return a + i*v })
			},
			model: func(in []int) any {
				acc := 0
				for i, v := range in {
					acc += i * v
				}
				return acc
			},
		},
		{
			name:   "FoldWhile",
			covers: []string{"FoldWhile"},
			op: func(s catena.Seq[int]) any {
				return s.FoldWhile(0, func(a, v int) (int, bool) { a += v; return a, a < 6 })
			},
			model: func(in []int) any {
				acc := 0
				for _, v := range in {
					acc += v
					if acc >= 6 {
						break
					}
				}
				return acc
			},
			infOp: func(s catena.Seq[int]) any {
				return s.FoldWhile(0, func(a, v int) (int, bool) { a += v; return a, a < 6 })
			},
		},
		{
			name:   "FoldErr",
			covers: []string{"FoldErr"},
			op: func(s catena.Seq[int]) any {
				acc, err := s.FoldErr(0, func(a, v int) (int, error) {
					if v == 3 {
						return 0, errBoom
					}
					return a + v, nil
				})
				return pair2{acc, err}
			},
			model: func(in []int) any {
				acc := 0
				for _, v := range in {
					if v == 3 {
						return pair2{acc, error(errBoom)}
					}
					acc += v
				}
				return pair2{acc, error(nil)}
			},
			infOp: func(s catena.Seq[int]) any {
				_, err := s.FoldErr(0, func(a, v int) (int, error) {
					if v == 3 {
						return 0, errBoom
					}
					return a + v, nil
				})
				return err
			},
		},
		{
			name:   "FoldBy",
			covers: []string{"FoldBy"},
			op: func(s catena.Seq[int]) any {
				return s.FoldBy(
					func(v int) int { return ((v % 3) + 3) % 3 },
					func(k int) int { return k * 100 },
					func(a, v int) int { return a + v },
				)
			},
			model: func(in []int) any {
				out := map[int]int{}
				for _, v := range in {
					k := ((v % 3) + 3) % 3
					if _, ok := out[k]; !ok {
						out[k] = k * 100
					}
					out[k] += v
				}
				return out
			},
		},
		{
			name:   "Reduce",
			covers: []string{"Reduce"},
			op: func(s catena.Seq[int]) any {
				v, ok := s.Reduce(func(a, b int) int { return a + b })
				return pair2{v, ok}
			},
			model: func(in []int) any {
				if len(in) == 0 {
					return pair2{0, false}
				}
				acc := in[0]
				for _, v := range in[1:] {
					acc += v
				}
				return pair2{acc, true}
			},
		},
		{
			name:   "GroupBy",
			covers: []string{"GroupBy"},
			op:     func(s catena.Seq[int]) any { return s.GroupBy(func(v int) int { return ((v % 3) + 3) % 3 }) },
			model: func(in []int) any {
				out := map[int][]int{}
				for _, v := range in {
					k := ((v % 3) + 3) % 3
					out[k] = append(out[k], v)
				}
				return out
			},
		},
		{
			name:   "IndexBy",
			covers: []string{"IndexBy"},
			op:     func(s catena.Seq[int]) any { return s.IndexBy(func(v int) int { return ((v % 3) + 3) % 3 }) },
			model: func(in []int) any {
				out := map[int]int{}
				for _, v := range in {
					out[((v%3)+3)%3] = v // last wins
				}
				return out
			},
		},
		{
			name:   "TallyBy",
			covers: []string{"TallyBy"},
			op:     func(s catena.Seq[int]) any { return s.TallyBy(even) },
			model: func(in []int) any {
				out := map[bool]int{}
				for _, v := range in {
					out[even(v)]++
				}
				return out
			},
		},
		{
			name:   "Associate",
			covers: []string{"Associate"},
			op: func(s catena.Seq[int]) any {
				return s.Associate(func(v int) (int, int) { return ((v % 3) + 3) % 3, v * 10 })
			},
			model: func(in []int) any {
				out := map[int]int{}
				for _, v := range in {
					out[((v%3)+3)%3] = v * 10 // last wins
				}
				return out
			},
		},
		{
			name:   "Partition",
			covers: []string{"Partition"},
			op: func(s catena.Seq[int]) any {
				yes, no := s.Partition(even)
				return pair2{yes, no}
			},
			model: func(in []int) any {
				var yes, no []int
				for _, v := range in {
					if even(v) {
						yes = append(yes, v)
					} else {
						no = append(no, v)
					}
				}
				return pair2{yes, no}
			},
		},
	}...)
}
