package parallel_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/parallel"
)

// What a stage costs when the callback costs nothing: the price of the
// goroutine per element, which is what an errgroup-style loop pays too.
// These exist to put a number on it, so the docs can say when parallel is and
// is not worth reaching for.

var benchInput = catena.Range(0, 10_000, 1).Collect()

var benchSink int

func double(_ context.Context, v int) (int, error) { return v * 2, nil }

func BenchmarkMap_Sequential(b *testing.B) {
	s := catena.FromSlice(benchInput)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range s.MapErr(func(v int) (int, error) { return v * 2, nil }) {
			n += v
		}
		benchSink = n
	}
}

// The loop a careful person writes by hand: a goroutine per element behind
// a semaphore, results into a preallocated slice.
func BenchmarkMap_Hand(b *testing.B) {
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		out := make([]int, len(benchInput))
		sem := make(chan struct{}, 8)
		var wg sync.WaitGroup
		for i, v := range benchInput {
			sem <- struct{}{}
			wg.Go(func() {
				out[i], _ = double(ctx, v)
				<-sem
			})
		}
		wg.Wait()
		n := 0
		for _, v := range out {
			n += v
		}
		benchSink = n
	}
}

func BenchmarkMap_Par(b *testing.B) {
	stage := catena.FromSlice(benchInput).Then(parallel.Map(context.Background(), 8, double))
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range stage {
			n += v
		}
		benchSink = n
	}
}

func BenchmarkMapUnordered_Par(b *testing.B) {
	stage := catena.FromSlice(benchInput).Then(parallel.MapUnordered(context.Background(), 8, double))
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range stage {
			n += v
		}
		benchSink = n
	}
}

func BenchmarkBuffer(b *testing.B) {
	stage := catena.FromSlice(benchInput).Then(parallel.Buffer[int](64))
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range stage {
			n += v
		}
		benchSink = n
	}
}

func BenchmarkForEach_Par(b *testing.B) {
	stage := parallel.ForEach(context.Background(), 8, func(context.Context, int) error { return nil })
	s := catena.FromSlice(benchInput)
	b.ReportAllocs()
	for b.Loop() {
		if err := s.Then(stage); err != nil {
			b.Fatal(err)
		}
	}
}

// ChunkedWithin against Chunked: what batching by time costs over
// batching by size alone, which is one channel hand-off per element.
func BenchmarkChunked_BySize(b *testing.B) {
	s := catena.FromSlice(benchInput)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for chunk := range catena.Chunked(s, 100) {
			n += len(chunk)
		}
		benchSink = n
	}
}

func BenchmarkChunkedWithin(b *testing.B) {
	stage := catena.FromSlice(benchInput).Then(parallel.ChunkedWithin[int](100, time.Second))
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for chunk := range stage {
			n += len(chunk)
		}
		benchSink = n
	}
}

func BenchmarkMerge(b *testing.B) {
	half := catena.FromSlice(benchInput[:len(benchInput)/2])
	merged := parallel.Merge(half, half)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v := range merged {
			n += v
		}
		benchSink = n
	}
}
