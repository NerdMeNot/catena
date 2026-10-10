package par_test

// The conformance harness for package par, mirroring the core package's.
// Every exported operator is registered here and run through the checks
// its execution model allows (P1-P16); completeness_test.go fails the
// build for one that is not.
//
// Everything runs inside a testing/synctest bubble. That gives virtual
// time, so timing assertions are exact rather than tolerant — and it makes
// a leak fatal twice over: each check asserts the stage's goroutines had
// finished at the moment its iteration returned, and synctest.Test itself
// fails if any goroutine is still blocked when the check ends.

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/par"
)

// ---- registries ----

type callback = func(context.Context, int) (int, error)

// mapOp is a caller-driven stage: Map or MapUnordered.
type mapOp struct {
	name    string
	covers  []string
	build   func(ctx context.Context, n int, f callback) func(catena.Seq[int]) catena.Try[int]
	ordered bool
}

var mapOps = []mapOp{
	{name: "Map", covers: []string{"Map"}, build: par.Map[int, int], ordered: true},
	{name: "MapUnordered", covers: []string{"MapUnordered"}, build: par.MapUnordered[int, int]},
}

// stageOp is a producer stage, flattened to Seq[int] -> Seq[int].
type stageOp struct {
	name   string
	covers []string
	op     func(catena.Seq[int]) catena.Seq[int]
	model  func([]int) []int
	ahead  int // P7: elements consumed beyond those yielded
}

var stageOps = []stageOp{
	{
		name:   "Buffer",
		covers: []string{"Buffer"},
		op:     func(s catena.Seq[int]) catena.Seq[int] { return s.Then(par.Buffer[int](3)) },
		model:  slices.Clone[[]int],
		ahead:  4, // n buffered + one in the producer's hand
	},
	{
		name:   "ChunkedWithin_FlattenSlices",
		covers: []string{"ChunkedWithin"},
		op: func(s catena.Seq[int]) catena.Seq[int] {
			return catena.FlattenSlices(s.Then(par.ChunkedWithin[int](3, time.Second)))
		},
		model: slices.Clone[[]int], // chunk then flatten is the identity
		ahead: 2,                   // the rest of the chunk + one in the producer's hand
	},
	{
		name:   "Merge",
		covers: []string{"Merge"},
		op:     func(s catena.Seq[int]) catena.Seq[int] { return par.Merge(s, nil) },
		model:  slices.Clone[[]int], // one live input: its own order
		ahead:  1,
	},
}

// dedicatedCovered names the exported symbols proven by a test of their
// own rather than through a registry.
var dedicatedCovered = map[string]string{
	"ForEach": "TestForEach",
}

// ---- fixtures ----

var conformInputs = [][]int{
	nil,
	{5},
	{1, 2, 3, 4, 5},
	{2, 2, 1, 3, 2, 2, 1},
	{9, 8, 7, 6, 5, 4, 3, 2, 1, 0, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0},
}

var errBoom = errors.New("boom")

// bubble runs f as a subtest inside a synctest bubble.
func bubble(t *testing.T, name string, f func(t *testing.T)) {
	t.Helper()
	t.Run(name, func(t *testing.T) { synctest.Test(t, f) })
}

// goid is the current goroutine's id — test-only, to prove which
// goroutine a callback ran on.
func goid() int {
	var buf [64]byte
	fields := strings.Fields(string(buf[:runtime.Stack(buf[:], false)]))
	id, _ := strconv.Atoi(fields[1])
	return id
}

// probe instruments a callback: how many calls started and finished, the
// most that ran at once, and how many saw their context cancelled.
type probe struct {
	started, finished, cur, max, cancelled atomic.Int64
}

func (p *probe) wrap(f callback) callback {
	return func(ctx context.Context, v int) (int, error) {
		p.started.Add(1)
		c := p.cur.Add(1)
		for m := p.max.Load(); c > m && !p.max.CompareAndSwap(m, c); m = p.max.Load() {
		}
		defer func() {
			if ctx.Err() != nil {
				p.cancelled.Add(1)
			}
			p.cur.Add(-1)
			p.finished.Add(1)
		}()
		return f(ctx, v)
	}
}

// joined reports whether every call that started has finished. Checked
// with no synctest.Wait first, it is the proof that the stage joined its
// goroutines before returning: a call's return precedes its goroutine's.
func (p *probe) joined() bool { return p.started.Load() == p.finished.Load() }

// scrambled is the standard callback: it finishes after a per-element
// virtual delay, so completion order differs from source order, and it
// fails every fourth value while returning a non-zero value alongside
// the error — which the stage must not let through.
func scrambled(_ context.Context, v int) (int, error) {
	time.Sleep(time.Duration(((v*7919)%5+5)%5) * time.Millisecond)
	if v%4 == 0 {
		return 999, errBoom
	}
	return v * 2, nil
}

type pair struct {
	v      int
	failed bool
}

func modelScrambled(in []int) []pair {
	var out []pair
	for _, v := range in {
		if v%4 == 0 {
			out = append(out, pair{0, true})
		} else {
			out = append(out, pair{v * 2, false})
		}
	}
	return out
}

func collectPairs(t catena.Try[int]) []pair {
	var out []pair
	for v, err := range t {
		out = append(out, pair{v, err != nil})
	}
	return out
}

func sortPairs(ps []pair) []pair {
	ps = slices.Clone(ps)
	slices.SortFunc(ps, func(a, b pair) int {
		if a.v != b.v {
			return a.v - b.v
		}
		return strings.Compare(fmt.Sprint(a.failed), fmt.Sprint(b.failed))
	})
	return ps
}

// source is an instrumented finite or infinite sequence: it counts
// iterations begun, cleanups run and elements produced, records which
// goroutines called it, and can pause or panic at a given element.
type source struct {
	n        int // elements, or -1 for infinite
	panicAt  int // element at which to panic, or 0 for never
	delay    func(i int) time.Duration
	starts   atomic.Int64
	cleanups atomic.Int64
	produced atomic.Int64
	goids    atomic.Value // []int, appended under the single-producer rule
}

func (src *source) seq() catena.Seq[int] {
	return func(yield func(int) bool) {
		src.starts.Add(1)
		defer src.cleanups.Add(1)
		ids, _ := src.goids.Load().([]int)
		src.goids.Store(append(slices.Clone(ids), goid()))
		for i := 0; src.n < 0 || i < src.n; i++ {
			if src.delay != nil {
				time.Sleep(src.delay(i))
			}
			if src.panicAt != 0 && i == src.panicAt {
				panic("source boom")
			}
			src.produced.Add(1)
			if !yield(i) {
				return
			}
		}
	}
}

func (src *source) clean() bool { return src.starts.Load() == src.cleanups.Load() }

// singleUse is a sequence that panics if iterated twice (P13).
func singleUse(n int) catena.Seq[int] {
	used := false
	return func(yield func(int) bool) {
		if used {
			panic("single-use source iterated twice")
		}
		used = true
		for i := range n {
			if !yield(i) {
				return
			}
		}
	}
}

func mustPanic(t *testing.T, f func()) (v any) {
	t.Helper()
	defer func() {
		if v = recover(); v == nil {
			t.Fatal("expected a panic")
		}
	}()
	f()
	return nil
}

// ---- caller-driven stages ----

func TestConformMap(t *testing.T) {
	for _, c := range mapOps {
		t.Run(c.name, func(t *testing.T) { conformMap(t, c) })
	}
}

func conformMap(t *testing.T, c mapOp) {
	ctx := context.Background

	bubble(t, "P1_nil_source", func(t *testing.T) {
		if got := collectPairs(c.build(ctx(), 2, scrambled)(nil)); got != nil {
			t.Fatalf("nil source yielded %v", got)
		}
	})

	bubble(t, "P2_P3_model_and_reiteration", func(t *testing.T) {
		for _, n := range []int{1, 2, 3, 8} {
			for _, in := range conformInputs {
				var p probe
				stage := c.build(ctx(), n, p.wrap(scrambled))(catena.FromSlice(in))
				first, second := collectPairs(stage), collectPairs(stage)
				if !p.joined() {
					t.Fatalf("n=%d %v: calls still running after the iteration returned", n, in)
				}
				want := modelScrambled(in)
				if !c.ordered {
					first, second, want = sortPairs(first), sortPairs(second), sortPairs(want)
				}
				if !slices.Equal(first, want) {
					t.Fatalf("n=%d %v: got %v, want %v", n, in, first, want)
				}
				if !slices.Equal(second, want) {
					t.Fatalf("n=%d %v: second iteration got %v, want %v", n, in, second, want)
				}
			}
		}
	})

	bubble(t, "P4_P5_early_break_joins", func(t *testing.T) {
		var p probe
		src := source{n: -1}
		f := p.wrap(func(ctx context.Context, v int) (int, error) {
			if v == 0 {
				return 0, nil
			}
			<-ctx.Done() // every other call runs until it is cancelled
			return 0, ctx.Err()
		})
		for range c.build(ctx(), 4, f)(src.seq()) {
			break
		}
		if !p.joined() {
			t.Fatal("calls still running after the break returned")
		}
		if got, want := p.cancelled.Load(), p.started.Load()-1; got != want {
			t.Fatalf("%d of the %d abandoned calls saw their context cancelled", got, want)
		}
		if !src.clean() {
			t.Fatal("source cleanup did not run on an early break")
		}
	})

	bubble(t, "P6_yield_discipline", func(t *testing.T) {
		for _, stopAt := range []int{1, 2} {
			for _, size := range []int{2, 20} { // shorter and longer than the window
				src := source{n: size}
				caller, calls, dead := goid(), 0, false
				c.build(ctx(), 4, scrambled)(src.seq())(func(int, error) bool {
					if dead {
						t.Fatal("yield called after it returned false")
					}
					if goid() != caller {
						t.Fatal("yield called off the consumer's goroutine")
					}
					calls++
					dead = calls == stopAt
					return !dead
				})
				if calls != stopAt {
					t.Fatalf("stopAt=%d size=%d: %d yields", stopAt, size, calls)
				}
				if ids := src.goids.Load().([]int); !slices.Equal(ids, []int{caller}) {
					t.Fatal("source ran off the consumer's goroutine")
				}
			}
		}
	})

	bubble(t, "P7_laziness", func(t *testing.T) {
		const k, n = 2, 3
		var p probe
		src := source{n: -1}
		yielded := 0
		for range c.build(ctx(), n, p.wrap(scrambled))(src.seq()) {
			if yielded++; yielded == k {
				break
			}
		}
		if got := src.produced.Load(); got > k+n {
			t.Fatalf("yielding %d consumed %d, more than k+n=%d", k, got, k+n)
		}
		if got := p.started.Load(); got > k+n {
			t.Fatalf("yielding %d started %d calls, more than k+n=%d", k, got, k+n)
		}
	})

	bubble(t, "P8_in_flight_bound", func(t *testing.T) {
		for _, tc := range []struct{ n, size, wantMax, wantSeconds int }{
			{n: 3, size: 10, wantMax: 3, wantSeconds: 4},
			{n: 8, size: 5, wantMax: 5, wantSeconds: 1},
			{n: 1, size: 3, wantMax: 1, wantSeconds: 3},
		} {
			var p probe
			f := p.wrap(func(_ context.Context, v int) (int, error) {
				time.Sleep(time.Second)
				return v, nil
			})
			begin := time.Now()
			got, err := c.build(ctx(), tc.n, f)(catena.Range(0, tc.size, 1)).Collect()
			if err != nil || len(got) != tc.size {
				t.Fatalf("%+v: got %v, %v", tc, got, err)
			}
			if m := p.max.Load(); m != int64(tc.wantMax) {
				t.Fatalf("%+v: %d calls ran at once", tc, m)
			}
			// Exact, in virtual time: the calls really do overlap.
			if d := time.Since(begin); d != time.Duration(tc.wantSeconds)*time.Second {
				t.Fatalf("%+v: took %v", tc, d)
			}
		}
	})

	bubble(t, "P9_head_of_line", func(t *testing.T) {
		f := func(_ context.Context, v int) (int, error) {
			if v == 0 {
				time.Sleep(10 * time.Second)
			}
			return v, nil
		}
		begin := time.Now()
		var firstAt time.Duration
		var got []int
		for v := range c.build(ctx(), 3, f)(catena.Range(0, 6, 1)) {
			if got = append(got, v); len(got) == 1 {
				firstAt = time.Since(begin)
			}
		}
		if c.ordered {
			// Source order, so nothing can pass the slow head.
			if !slices.Equal(got, []int{0, 1, 2, 3, 4, 5}) || firstAt != 10*time.Second {
				t.Fatalf("got %v, first at %v", got, firstAt)
			}
			return
		}
		// Completion order, so the slow call delays only itself.
		if got[len(got)-1] != 0 || firstAt != 0 {
			t.Fatalf("got %v, first at %v", got, firstAt)
		}
	})

	untilCancelled := func(ctx context.Context, _ int) (int, error) {
		<-ctx.Done()
		return 0, ctx.Err()
	}

	// lingering is a call that is slow to stop: it outlives its own
	// cancellation by a second. A stage that cancels without waiting
	// returns while these are still running, which joined() then sees —
	// deterministically, since virtual time cannot pass while the test
	// goroutine is running.
	lingering := func(ctx context.Context, _ int) (int, error) {
		<-ctx.Done()
		time.Sleep(time.Second)
		return 0, ctx.Err()
	}

	bubble(t, "P10a_cancel_between_elements", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx())
		defer cancel()
		var p probe
		var errs []error
		values := 0
		for _, err := range c.build(cctx, 2, p.wrap(scrambled))(catena.Range(1, 100, 4)) {
			if errs = append(errs, err); err == nil {
				values++
				cancel()
			}
		}
		// One value, then exactly one error, the context's, and no more.
		if values != 1 || len(errs) != 2 || !errors.Is(errs[1], context.Canceled) {
			t.Fatalf("values=%d errs=%v", values, errs)
		}
		if !p.joined() {
			t.Fatal("calls still running after cancellation")
		}
	})

	bubble(t, "P10b_cancel_while_blocked", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx())
		go func() {
			time.Sleep(time.Second)
			cancel()
		}()
		var p probe
		src := source{n: 5}
		got, err := c.build(cctx, 2, p.wrap(untilCancelled))(src.seq()).Collect()
		if got != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, %v", got, err)
		}
		if !p.joined() || !src.clean() {
			t.Fatal("not joined, or source not cleaned up, after cancellation")
		}
	})

	bubble(t, "P10c_deadline", func(t *testing.T) {
		cctx, cancel := context.WithTimeout(ctx(), time.Second)
		defer cancel()
		begin := time.Now()
		_, err := c.build(cctx, 2, untilCancelled)(catena.Range(0, 5, 1)).Collect()
		if !errors.Is(err, context.DeadlineExceeded) || time.Since(begin) != time.Second {
			t.Fatalf("err=%v after %v", err, time.Since(begin))
		}
	})

	bubble(t, "P10d_already_cancelled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx())
		cancel()
		var p probe
		stage := c.build(cctx, 2, p.wrap(scrambled))
		_, errs := stage(catena.Of(1, 2, 3)).CollectAll()
		if len(errs) != 1 || !errors.Is(errs[0], context.Canceled) || p.started.Load() != 0 {
			t.Fatalf("errs=%v, calls started=%d", errs, p.started.Load())
		}
		// An empty source has nothing to report the cancellation on.
		if got := collectPairs(stage(catena.Empty[int]())); got != nil {
			t.Fatalf("empty source yielded %v", got)
		}
	})

	bubble(t, "P10e_cancel_during_final_drain", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx())
		go func() {
			time.Sleep(500 * time.Millisecond)
			cancel()
		}()
		slow := func(_ context.Context, v int) (int, error) {
			time.Sleep(time.Second) // deliberately ignores its context
			return v, nil
		}
		// The source has ended, so the stage is waiting on its last call.
		got, err := c.build(cctx, 4, slow)(catena.Of(1)).Collect()
		if got != nil || !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v, %v", got, err)
		}
	})

	bubble(t, "P11_callback_panic", func(t *testing.T) {
		var p probe
		src := source{n: 10}
		f := p.wrap(func(ctx context.Context, v int) (int, error) {
			switch {
			case v < 2:
				return v, nil
			case v == 2:
				panic("callback boom")
			}
			return lingering(ctx, v)
		})
		v := mustPanic(t, func() { c.build(ctx(), 4, f)(src.seq()).Collect() })
		if v != "callback boom" {
			t.Fatalf("recovered %v", v)
		}
		if !p.joined() || !src.clean() {
			t.Fatal("not joined, or source not cleaned up, when the panic surfaced")
		}
	})

	bubble(t, "P11_consumer_panic", func(t *testing.T) {
		var p probe
		f := p.wrap(func(ctx context.Context, v int) (int, error) {
			if v == 0 {
				return v, nil
			}
			return lingering(ctx, v)
		})
		v := mustPanic(t, func() {
			for range c.build(ctx(), 4, f)(catena.Range(0, 10, 1)) {
				panic("consumer boom")
			}
		})
		if v != "consumer boom" || !p.joined() {
			t.Fatalf("recovered %v, joined=%v", v, p.joined())
		}
	})

	bubble(t, "P11_panic_after_break", func(t *testing.T) {
		// A call that panics after the consumer has stopped is abandoned
		// work, but its panic is still not swallowed.
		f := func(ctx context.Context, v int) (int, error) {
			if v == 0 {
				return 0, nil
			}
			<-ctx.Done()
			panic("late boom")
		}
		src := source{n: -1}
		v := mustPanic(t, func() {
			for range c.build(ctx(), 3, f)(src.seq()) {
				break
			}
		})
		if v != "late boom" {
			t.Fatalf("recovered %v", v)
		}
	})

	bubble(t, "P11_goexit", func(t *testing.T) {
		f := func(_ context.Context, v int) (int, error) {
			if v == 1 {
				runtime.Goexit()
			}
			return v, nil
		}
		v := mustPanic(t, func() { c.build(ctx(), 2, f)(catena.Range(0, 5, 1)).Collect() })
		if err, ok := v.(error); !ok || !strings.Contains(err.Error(), "runtime.Goexit") {
			t.Fatalf("recovered %v", v)
		}
	})

	bubble(t, "P12_source_panic", func(t *testing.T) {
		var p probe
		src := source{n: 10, panicAt: 3}
		v := mustPanic(t, func() { c.build(ctx(), 4, p.wrap(lingering))(src.seq()).Collect() })
		if v != "source boom" || !p.joined() {
			t.Fatalf("recovered %v, joined=%v", v, p.joined())
		}
	})

	bubble(t, "P13_single_consumption", func(t *testing.T) {
		if got := collectPairs(c.build(ctx(), 2, scrambled)(singleUse(5))); len(got) != 5 {
			t.Fatalf("got %v", got)
		}
	})

	bubble(t, "P14_errors_do_not_stop", func(t *testing.T) {
		var p probe
		failing := func(context.Context, int) (int, error) { return 7, errBoom }
		vals, errs := c.build(ctx(), 3, p.wrap(failing))(catena.Range(0, 9, 1)).CollectAll()
		if vals != nil || len(errs) != 9 || p.started.Load() != 9 {
			t.Fatalf("vals=%v errs=%d calls=%d", vals, len(errs), p.started.Load())
		}
	})
}

func TestForEach(t *testing.T) {
	ctx := context.Background()

	bubble(t, "all_succeed", func(t *testing.T) {
		var sum atomic.Int64
		var p probe
		err := catena.Range(1, 101, 1).Then(par.ForEach(ctx, 8, func(ctx context.Context, v int) error {
			_, err := p.wrap(func(context.Context, int) (int, error) {
				time.Sleep(time.Millisecond)
				sum.Add(int64(v))
				return 0, nil
			})(ctx, v)
			return err
		}))
		if err != nil || sum.Load() != 5050 || p.max.Load() != 8 {
			t.Fatalf("err=%v sum=%d max=%d", err, sum.Load(), p.max.Load())
		}
	})

	bubble(t, "first_error_cancels_the_rest", func(t *testing.T) {
		var p probe
		src := source{n: -1}
		f := p.wrap(func(ctx context.Context, v int) (int, error) {
			if v == 2 {
				return 0, errBoom
			}
			<-ctx.Done()
			return 0, ctx.Err()
		})
		err := src.seq().Then(par.ForEach(ctx, 4, func(ctx context.Context, v int) error {
			_, err := f(ctx, v)
			return err
		}))
		// The failing call's error, not a sibling's cancellation.
		if !errors.Is(err, errBoom) || !p.joined() || !src.clean() {
			t.Fatalf("err=%v joined=%v clean=%v", err, p.joined(), src.clean())
		}
	})

	bubble(t, "parent_cancelled", func(t *testing.T) {
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		err := catena.Of(1).Then(par.ForEach(cctx, 2, func(context.Context, int) error { return nil }))
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v", err)
		}
	})

	bubble(t, "nil_source", func(t *testing.T) {
		var s catena.Seq[int]
		if err := s.Then(par.ForEach(ctx, 2, func(context.Context, int) error { return errBoom })); err != nil {
			t.Fatalf("err=%v", err)
		}
	})
}

// ---- producer stages ----

func TestConformStage(t *testing.T) {
	for _, c := range stageOps {
		t.Run(c.name, func(t *testing.T) { conformStage(t, c) })
	}
}

func conformStage(t *testing.T, c stageOp) {
	bubble(t, "P1_nil_source", func(t *testing.T) {
		if got := c.op(nil).Collect(); got != nil {
			t.Fatalf("nil source yielded %v", got)
		}
	})

	bubble(t, "P2_P3_model_and_reiteration", func(t *testing.T) {
		for _, in := range conformInputs {
			stage := c.op(catena.FromSlice(in))
			want := c.model(in)
			if got := stage.Collect(); !slices.Equal(got, want) {
				t.Fatalf("%v: got %v, want %v", in, got, want)
			}
			if got := stage.Collect(); !slices.Equal(got, want) {
				t.Fatalf("%v: second iteration got %v, want %v", in, got, want)
			}
		}
	})

	bubble(t, "P4_P5_P7_early_break_joins", func(t *testing.T) {
		const k = 2
		src := source{n: -1}
		yielded := 0
		for range c.op(src.seq()) {
			if yielded++; yielded == k {
				break
			}
		}
		// No Wait: the producer must already have returned.
		if !src.clean() {
			t.Fatal("the source was still running after the break returned")
		}
		if got := src.produced.Load(); got > int64(k+c.ahead) {
			t.Fatalf("yielding %d consumed %d, more than k+%d", k, got, c.ahead)
		}
	})

	bubble(t, "P6_yield_discipline", func(t *testing.T) {
		for _, stopAt := range []int{1, 2} {
			src := source{n: 20}
			caller, calls, dead := goid(), 0, false
			c.op(src.seq())(func(int) bool {
				if dead {
					t.Fatal("yield called after it returned false")
				}
				if goid() != caller {
					t.Fatal("yield called off the consumer's goroutine")
				}
				calls++
				dead = calls == stopAt
				return !dead
			})
			// The source runs on one goroutine, and it is not ours.
			if ids := src.goids.Load().([]int); len(ids) != 1 || ids[0] == caller {
				t.Fatalf("source ran on goroutines %v, consumer is %d", ids, caller)
			}
		}
	})

	bubble(t, "P12_source_panic", func(t *testing.T) {
		src := source{n: 10, panicAt: 4}
		var got []int
		v := mustPanic(t, func() {
			for v := range c.op(src.seq()) {
				got = append(got, v)
			}
		})
		// Re-raised on the consumer, with the value it was raised with.
		if v != "source boom" || !src.clean() {
			t.Fatalf("recovered %v, clean=%v", v, src.clean())
		}
		if len(got) > 4 || !slices.Equal(got, []int{0, 1, 2, 3}[:len(got)]) {
			t.Fatalf("yielded %v before the panic", got)
		}
	})

	bubble(t, "P13_single_consumption", func(t *testing.T) {
		if got := c.op(singleUse(5)).Collect(); !slices.Equal(got, c.model([]int{0, 1, 2, 3, 4})) {
			t.Fatalf("got %v", got)
		}
	})

	bubble(t, "P16_stop_while_producer_is_parked", func(t *testing.T) {
		src := source{n: -1}
		for range c.op(src.seq()) {
			synctest.Wait() // the producer is now blocked on its send
			break
		}
		if !src.clean() {
			t.Fatal("a parked producer was not released by the break")
		}
	})
}

func TestBufferRunsAhead(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// A source and a consumer that each take a second per element
		// overlap behind a buffer instead of taking turns.
		src := source{n: 5, delay: func(int) time.Duration { return time.Second }}
		begin := time.Now()
		for range src.seq().Then(par.Buffer[int](2)) {
			time.Sleep(time.Second)
		}
		if d := time.Since(begin); d != 6*time.Second {
			t.Fatalf("took %v; taking turns would be 10s", d)
		}

		// And it runs at most n+1 ahead of a consumer that has stopped
		// taking: n in the buffer and one in the producer's hand.
		ahead := source{n: -1}
		for range ahead.seq().Then(par.Buffer[int](3)) {
			synctest.Wait()
			if got := ahead.produced.Load(); got != 1+3+1 {
				t.Fatalf("produced %d with one yielded", got)
			}
			break
		}
	})
}

func TestMerge(t *testing.T) {
	bubble(t, "interleaves_by_arrival", func(t *testing.T) {
		every := func(d time.Duration, offset int) catena.Seq[int] {
			src := &source{n: 3, delay: func(int) time.Duration { return d }}
			return src.seq().Map(func(v int) int { return v + offset })
		}
		// Arrivals at 2s, 4s, 6s and at 5s, 10s, 15s: never together.
		got := par.Merge(every(2*time.Second, 0), every(5*time.Second, 100)).Collect()
		if !slices.Equal(got, []int{0, 1, 100, 2, 101, 102}) {
			t.Fatalf("got %v", got)
		}
	})

	bubble(t, "runs_sources_concurrently", func(t *testing.T) {
		slow := func() catena.Seq[int] {
			src := &source{n: 4, delay: func(int) time.Duration { return time.Second }}
			return src.seq()
		}
		begin := time.Now()
		got := par.Merge(slow(), slow(), slow()).Collect()
		if len(got) != 12 || time.Since(begin) != 4*time.Second {
			t.Fatalf("%d elements in %v", len(got), time.Since(begin))
		}
	})

	bubble(t, "no_sources", func(t *testing.T) {
		if got := par.Merge[int]().Collect(); got != nil {
			t.Fatalf("got %v", got)
		}
	})

	bubble(t, "break_stops_every_source", func(t *testing.T) {
		a, b, c := source{n: -1}, source{n: -1}, source{n: -1}
		for range par.Merge(a.seq(), b.seq(), c.seq()) {
			break
		}
		if !a.clean() || !b.clean() || !c.clean() {
			t.Fatal("a source was still running after the break returned")
		}
	})

	bubble(t, "one_panic_stops_the_rest", func(t *testing.T) {
		// Without the abort, the infinite sibling would keep the panic
		// from ever surfacing.
		bad, endless := source{n: 10, panicAt: 2}, source{n: -1}
		v := mustPanic(t, func() { par.Merge(endless.seq(), bad.seq()).Drain() })
		if v != "source boom" || !endless.clean() {
			t.Fatalf("recovered %v, sibling clean=%v", v, endless.clean())
		}
	})

	bubble(t, "source_goexit", func(t *testing.T) {
		exits := catena.Seq[int](func(func(int) bool) { runtime.Goexit() })
		v := mustPanic(t, func() { par.Merge(exits).Drain() })
		if err, ok := v.(error); !ok || !strings.Contains(err.Error(), "runtime.Goexit") {
			t.Fatalf("recovered %v", v)
		}
	})
}

// P15: ChunkedWithin's timing, exactly, in virtual time.
func TestChunkedWithinTiming(t *testing.T) {
	type arrival struct {
		after time.Duration // since the previous arrival
		v     int
	}
	type emission struct {
		at    time.Duration
		chunk string
	}
	const ms = time.Millisecond
	for _, tc := range []struct {
		name     string
		n        int
		d        time.Duration
		arrivals []arrival
		linger   time.Duration // silence before the source ends
		want     []emission
	}{
		{
			name:     "full_chunk_does_not_wait",
			n:        3,
			d:        time.Second,
			arrivals: []arrival{{0, 1}, {0, 2}, {0, 3}, {0, 4}},
			linger:   5 * time.Second,
			want:     []emission{{0, "[1 2 3]"}, {time.Second, "[4]"}},
		},
		{
			name:     "deadline_flushes_a_partial_chunk",
			n:        10,
			d:        time.Second,
			arrivals: []arrival{{0, 1}, {3 * time.Second, 2}},
			want:     []emission{{time.Second, "[1]"}, {3 * time.Second, "[2]"}},
		},
		{
			name:     "window_opens_at_the_first_element",
			n:        10,
			d:        time.Second,
			arrivals: []arrival{{0, 1}, {600 * ms, 2}, {600 * ms, 3}},
			linger:   5 * time.Second,
			// Not a sliding window: 2 does not extend 1's deadline.
			want: []emission{{time.Second, "[1 2]"}, {2200 * ms, "[3]"}},
		},
		{
			name:     "silence_yields_nothing",
			n:        10,
			d:        time.Second,
			arrivals: []arrival{{5 * time.Second, 1}},
			linger:   3 * time.Second,
			want:     []emission{{6 * time.Second, "[1]"}},
		},
		{
			name:     "source_end_flushes_the_remainder",
			n:        2,
			d:        time.Hour,
			arrivals: []arrival{{0, 1}, {0, 2}, {0, 3}},
			want:     []emission{{0, "[1 2]"}, {0, "[3]"}},
		},
		{
			name:   "empty_source",
			n:      2,
			d:      time.Second,
			linger: 3 * time.Second,
		},
	} {
		bubble(t, tc.name, func(t *testing.T) {
			src := catena.Seq[int](func(yield func(int) bool) {
				for _, a := range tc.arrivals {
					time.Sleep(a.after)
					if !yield(a.v) {
						return
					}
				}
				time.Sleep(tc.linger)
			})
			begin := time.Now()
			var got []emission
			for chunk := range src.Then(par.ChunkedWithin[int](tc.n, tc.d)) {
				got = append(got, emission{time.Since(begin), fmt.Sprint(chunk)})
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}

	bubble(t, "chunks_are_fresh", func(t *testing.T) {
		var kept [][]int
		for chunk := range catena.Range(0, 6, 1).Then(par.ChunkedWithin[int](2, time.Second)) {
			kept = append(kept, chunk)
			chunk[0] = -1 // scribbling on one must not reach the next
		}
		if fmt.Sprint(kept) != "[[-1 1] [-1 3] [-1 5]]" {
			t.Fatalf("got %v", kept)
		}
	})

	bubble(t, "panic_drops_the_partial_chunk", func(t *testing.T) {
		src := source{n: 10, panicAt: 2}
		var got [][]int
		v := mustPanic(t, func() {
			for chunk := range src.seq().Then(par.ChunkedWithin[int](5, time.Hour)) {
				got = append(got, chunk)
			}
		})
		if v != "source boom" || got != nil {
			t.Fatalf("recovered %v after yielding %v", v, got)
		}
	})

	bubble(t, "break_on_a_deadline_flush", func(t *testing.T) {
		src := source{n: -1, delay: func(int) time.Duration { return 3 * time.Second }}
		for range src.seq().Then(par.ChunkedWithin[int](10, time.Second)) {
			break
		}
		if !src.clean() {
			t.Fatal("the source was still running after the break returned")
		}
	})
}

func TestConstructionPanics(t *testing.T) {
	ctx := context.Background()
	f := func(context.Context, int) (int, error) { return 0, nil }
	for want, build := range map[string]func(){
		"par: Map: n must be > 0":           func() { par.Map(ctx, 0, f) },
		"par: MapUnordered: n must be > 0":  func() { par.MapUnordered(ctx, -1, f) },
		"par: ForEach: n must be > 0":       func() { par.ForEach(ctx, 0, func(context.Context, int) error { return nil }) },
		"par: Buffer: n must be > 0":        func() { par.Buffer[int](0) },
		"par: ChunkedWithin: n must be > 0": func() { par.ChunkedWithin[int](0, time.Second) },
		"par: ChunkedWithin: d must be > 0": func() { par.ChunkedWithin[int](1, 0) },
	} {
		if got := mustPanic(t, build); got != want {
			t.Errorf("panicked with %q, want %q", got, want)
		}
	}
}

// TestFixtures proves the harness can fail: a check that cannot catch the
// fault it exists for proves nothing by passing.
func TestFixtures(t *testing.T) {
	bubble(t, "joined_catches_a_leaked_call", func(t *testing.T) {
		var p probe
		release := make(chan struct{})
		f := p.wrap(func(context.Context, int) (int, error) {
			<-release
			return 0, nil
		})
		leaky := func() { go f(context.Background(), 1) } // starts a call and does not wait
		leaky()
		synctest.Wait()
		if p.joined() {
			t.Fatal("joined() did not notice a call still running")
		}
		close(release)
		synctest.Wait()
		if !p.joined() {
			t.Fatal("joined() still false after the call finished")
		}
	})

	bubble(t, "clean_catches_a_running_source", func(t *testing.T) {
		src := source{n: -1}
		next, stop := src.seq().Pull()
		next()
		if src.clean() {
			t.Fatal("clean() did not notice a source mid-iteration")
		}
		stop()
		if !src.clean() {
			t.Fatal("clean() still false after the source was stopped")
		}
	})

	bubble(t, "probe_measures_overlap", func(t *testing.T) {
		var p probe
		f := p.wrap(func(context.Context, int) (int, error) {
			time.Sleep(time.Second)
			return 0, nil
		})
		for range 5 {
			go f(context.Background(), 0)
		}
		time.Sleep(2 * time.Second)
		if p.max.Load() != 5 || p.cur.Load() != 0 {
			t.Fatalf("max=%d cur=%d", p.max.Load(), p.cur.Load())
		}
	})
}
