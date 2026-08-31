package catena_test

// Tests for constructors.go: every producer's contract, including the
// Range overflow guards, Cycle's empty-input termination, and the
// early-termination paths through each constructor's tail.

import (
	"context"
	"slices"
	"testing"

	"github.com/NerdMeNot/catena"
)

// ---- constructors ----

func TestRange(t *testing.T) {
	if got := catena.Range(0, 10, 3).Collect(); !slices.Equal(got, []int{0, 3, 6, 9}) {
		t.Fatalf("asc: %v", got)
	}
	if got := catena.Range(10, 0, -3).Collect(); !slices.Equal(got, []int{10, 7, 4, 1}) {
		t.Fatalf("desc: %v", got)
	}
	if got := catena.Range(0, 10, -1).Collect(); got != nil {
		t.Fatalf("sign mismatch must be empty: %v", got)
	}
	if got := catena.Range(10, 0, 1).Collect(); got != nil {
		t.Fatalf("sign mismatch must be empty: %v", got)
	}
	if got := catena.Range(5, 5, 1).Collect(); got != nil {
		t.Fatalf("empty range: %v", got)
	}
	t.Run("overflow_guard", func(t *testing.T) {
		mustComplete(t, testTimeout, func() {
			got := catena.Range(int8(120), int8(127), int8(5)).Collect()
			if !slices.Equal(got, []int8{120, 125}) {
				t.Fatalf("int8 overflow: %v", got)
			}
		})
		mustComplete(t, testTimeout, func() {
			got := catena.Range(int8(-120), int8(-128), int8(-5)).Collect()
			if !slices.Equal(got, []int8{-120, -125}) {
				t.Fatalf("int8 underflow: %v", got)
			}
		})
		mustComplete(t, testTimeout, func() {
			got := catena.Range(uint8(250), uint8(255), uint8(4)).Collect()
			if !slices.Equal(got, []uint8{250, 254}) {
				t.Fatalf("uint8 wrap: %v", got)
			}
		})
	})
	// re-iterable
	r := catena.Range(0, 3, 1)
	if a, b := r.Collect(), r.Collect(); !slices.Equal(a, b) {
		t.Fatal("Range not re-iterable")
	}
}

func TestGenerate(t *testing.T) {
	got := catena.Generate(1, func(v int) int { return v * 2 }).Take(4).Collect()
	if !slices.Equal(got, []int{1, 2, 4, 8}) { // seed yielded first
		t.Fatalf("got %v", got)
	}
}

func TestGenerateWhile(t *testing.T) {
	// a value produced alongside ok=false is NOT yielded
	got := catena.GenerateWhile(1, func(v int) (int, bool) { return v * 2, v < 4 }).Collect()
	if !slices.Equal(got, []int{1, 2, 4}) {
		t.Fatalf("got %v", got)
	}
	// seed is yielded unconditionally, even when next immediately stops
	got = catena.GenerateWhile(99, func(int) (int, bool) { return 0, false }).Collect()
	if !slices.Equal(got, []int{99}) {
		t.Fatalf("seed not yielded: %v", got)
	}
}

func TestRepeatAndOnce1AndEmpty(t *testing.T) {
	if got := catena.Repeat("x").Take(3).Collect(); !slices.Equal(got, []string{"x", "x", "x"}) {
		t.Fatalf("Repeat: %v", got)
	}
	if got := catena.RepeatN("y", 2).Collect(); !slices.Equal(got, []string{"y", "y"}) {
		t.Fatalf("RepeatN: %v", got)
	}
	if got := catena.Once1(7).Collect(); !slices.Equal(got, []int{7}) {
		t.Fatalf("Once1: %v", got)
	}
	// Once1 stops cleanly under early break
	for range catena.Once1(7) {
		break
	}
	if got := catena.Empty[int]().Collect(); got != nil {
		t.Fatalf("Empty: %v", got)
	}
	if got := collect2(catena.Empty2[int, string]()); got != nil {
		t.Fatalf("Empty2: %v", got)
	}
	if got := collectTry(catena.EmptyTry[int]()); got != nil {
		t.Fatalf("EmptyTry: %v", got)
	}
}

func TestCycle(t *testing.T) {
	if got := catena.Cycle(catena.Of(1, 2)).Take(5).Collect(); !slices.Equal(got, []int{1, 2, 1, 2, 1}) {
		t.Fatalf("got %v", got)
	}
	t.Run("empty_terminates", func(t *testing.T) {
		mustComplete(t, testTimeout, func() {
			if got := catena.Cycle(catena.Empty[int]()).Collect(); got != nil {
				t.Fatalf("got %v", got)
			}
		})
	})
	t.Run("nil_terminates", func(t *testing.T) {
		mustComplete(t, testTimeout, func() { catena.Cycle[int](nil).Drain() })
	})
	t.Run("reiterable", func(t *testing.T) {
		c := catena.Cycle(catena.Of(1, 2)).Take(3)
		if a, b := c.Collect(), c.Collect(); !slices.Equal(a, b) {
			t.Fatalf("%v then %v", a, b)
		}
	})
	t.Run("single_pass_source_buffered", func(t *testing.T) {
		// the replay buffer serves later cycles without re-reading the source
		c := catena.Cycle(catena.Seq[int](singleUse(2))).Take(6)
		if got := c.Collect(); !slices.Equal(got, []int{0, 1, 0, 1, 0, 1}) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestFromChan(t *testing.T) {
	t.Run("drains_until_close", func(t *testing.T) {
		ch := make(chan int, 3)
		ch <- 1
		ch <- 2
		ch <- 3
		close(ch)
		got := catena.FromChan(context.Background(), ch).Collect()
		if !slices.Equal(got, []int{1, 2, 3}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("stops_on_cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		ch := make(chan int)
		go func() {
			ch <- 1
			cancel()
		}()
		mustComplete(t, testTimeout, func() {
			got := catena.FromChan(ctx, ch).Collect()
			if !slices.Equal(got, []int{1}) {
				t.Fatalf("got %v", got)
			}
		})
	})
	t.Run("early_break", func(t *testing.T) {
		ch := make(chan int, 3)
		ch <- 1
		ch <- 2
		got := catena.FromChan(context.Background(), ch).Take(1).Collect()
		if !slices.Equal(got, []int{1}) {
			t.Fatalf("got %v", got)
		}
	})
}

func TestOfAndFromSliceSemantics(t *testing.T) {
	// FromSlice does not copy: later mutations are visible on re-iteration
	back := []int{1, 2, 3}
	s := catena.FromSlice(back)
	_ = s.Collect()
	back[0] = 99
	if got := s.Collect(); got[0] != 99 {
		t.Fatal("FromSlice must not copy (documented)")
	}
}

// Branch closure: consumer breaks inside each constructor's own loops.
func TestConstructorEarlyBreaks(t *testing.T) {
	t.Run("FromMap", func(t *testing.T) {
		s := catena.FromMap(map[int]string{1: "a", 2: "b", 3: "c"})
		if got := collect2(catena.Seq2[int, string](s).Take(1)); len(got) != 1 {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("RepeatN", func(t *testing.T) {
		if got := catena.RepeatN(1, 5).Take(2).Collect(); !slices.Equal(got, []int{1, 1}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("GenerateWhile", func(t *testing.T) {
		got := catena.GenerateWhile(1, func(v int) (int, bool) { return v + 1, true }).Take(2).Collect()
		if !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Range_desc_break", func(t *testing.T) {
		if got := catena.Range(10, 0, -1).Take(2).Collect(); !slices.Equal(got, []int{10, 9}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Cycle_break_in_first_pass", func(t *testing.T) {
		if got := catena.Cycle(catena.Of(1, 2, 3)).Take(2).Collect(); !slices.Equal(got, []int{1, 2}) {
			t.Fatalf("got %v", got)
		}
	})
	t.Run("Cycle_break_in_replay", func(t *testing.T) {
		// break while replaying the buffer (second lap), not the first pass
		if got := catena.Cycle(catena.Of(1, 2, 3)).Take(4).Collect(); !slices.Equal(got, []int{1, 2, 3, 1}) {
			t.Fatalf("got %v", got)
		}
	})
}
