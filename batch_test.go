package catena_test

// Conformance registrations for batch.go, via flatten round-trips (chunk
// then flatten is the identity), plus the window shapes and the L3
// fresh-slice guarantee that makes retaining an emitted batch safe.

import (
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

func init() {
	registerSeqOps([]seqOp{
		{
			name:   "Chunked_FlattenSlices",
			covers: []string{"Chunked", "FlattenSlices"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return catena.FlattenSlices(catena.Chunked(s, 3))
			},
			model: slices.Clone[[]int], // chunk then flatten is the identity
			alloc: allocBounded,
		},
		{
			name:   "ChunkedBy_FlattenSlices",
			covers: []string{"ChunkedBy"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return catena.FlattenSlices(catena.ChunkedBy(s, even))
			},
			model: slices.Clone[[]int],
			alloc: allocBounded,
		},
		{
			name:   "Windowed_FlattenSlices",
			covers: []string{"Windowed"},
			op: func(s catena.Seq[int]) catena.Seq[int] {
				return catena.FlattenSlices(catena.Windowed(s, 3, 3))
			},
			model: func(in []int) []int {
				return slices.Clone(in[:len(in)-len(in)%3]) // full windows only
			},
			alloc: allocBounded,
		},
	}...)
}

func TestWindowedShapes(t *testing.T) {
	collect := func(s catena.Seq[[]int]) [][]int {
		var out [][]int
		for w := range s.Seq() {
			out = append(out, w)
		}
		return out
	}
	s := catena.Range(0, 8, 1)
	t.Run("sliding", func(t *testing.T) {
		got := collect(catena.Windowed(s, 3, 1))
		want := [][]int{{0, 1, 2}, {1, 2, 3}, {2, 3, 4}, {3, 4, 5}, {4, 5, 6}, {5, 6, 7}}
		if !slices.EqualFunc(got, want, slices.Equal) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("step_gt_size_samples", func(t *testing.T) {
		got := collect(catena.Windowed(s, 2, 3))
		want := [][]int{{0, 1}, {3, 4}, {6, 7}}
		if !slices.EqualFunc(got, want, slices.Equal) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("partial_window_dropped", func(t *testing.T) {
		got := collect(catena.Windowed(catena.Of(1, 2, 3, 4, 5), 3, 3))
		want := [][]int{{1, 2, 3}}
		if !slices.EqualFunc(got, want, slices.Equal) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("windows_are_fresh_slices_L3", func(t *testing.T) {
		got := collect(catena.Windowed(s, 3, 1))
		got[0][0] = 999 // mutating a retained window must not corrupt others
		if got[1][0] != 1 {
			t.Fatalf("windows share backing memory: %v", got[:2])
		}
	})
	t.Run("chunks_are_fresh_slices_L3", func(t *testing.T) {
		got := collect(catena.Chunked(s, 2))
		got[0][0] = 999
		if got[1][0] != 2 {
			t.Fatalf("chunks share backing memory: %v", got[:2])
		}
	})
}

func TestChunkedByRuns(t *testing.T) {
	var out [][]int
	for c := range catena.ChunkedBy(catena.Of(1, 1, 2, 1, 1, 3), catena.Self[int]).Seq() {
		out = append(out, c)
	}
	want := [][]int{{1, 1}, {2}, {1, 1}, {3}}
	if !slices.EqualFunc(out, want, slices.Equal) {
		t.Fatalf("got %v", out)
	}
}

// Branch closure: consumer breaks between emitted batches.
func TestBatchEarlyBreaks(t *testing.T) {
	t.Run("Chunked_break_between_chunks", func(t *testing.T) {
		n := 0
		for range catena.Chunked(catena.Range(0, 100, 1), 3).Seq() {
			if n++; n == 2 {
				break
			}
		}
	})
	t.Run("ChunkedBy_break_between_chunks", func(t *testing.T) {
		n := 0
		for range catena.ChunkedBy(catena.Range(0, 100, 1), func(v int) int { return v / 2 }).Seq() {
			if n++; n == 2 {
				break
			}
		}
	})
}
