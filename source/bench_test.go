package source_test

import (
	"bufio"
	"io"
	"strings"
	"testing"

	"github.com/NerdMeNot/catena/source"
)

// Lines against the loop it replaces: bufio.Scanner over the same reader.

var benchText = strings.Repeat("2026-10-10T12:00:00Z INFO request handled in 12ms path=/api/v1/orders\n", 10_000)

var benchSink int

func BenchmarkLines_Scanner(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchText)))
	for b.Loop() {
		n := 0
		sc := bufio.NewScanner(strings.NewReader(benchText))
		for sc.Scan() {
			n += len(sc.Text())
		}
		benchSink = n
	}
}

func BenchmarkLines_Source(b *testing.B) {
	lines := source.Lines(func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(benchText)), nil
	})
	b.ReportAllocs()
	b.SetBytes(int64(len(benchText)))
	for b.Loop() {
		n := 0
		for line, err := range lines {
			if err != nil {
				b.Fatal(err)
			}
			n += len(line)
		}
		benchSink = n
	}
}

func BenchmarkPaginate_Hand(b *testing.B) {
	page := make([]int, 100)
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for cursor := 0; cursor < 100; cursor++ {
			for _, v := range page {
				n += v + cursor
			}
		}
		benchSink = n
	}
}

func BenchmarkPaginate_Source(b *testing.B) {
	page := make([]int, 100)
	pages := source.Paginate(0, func(c int) ([]int, int, bool, error) { return page, c + 1, c < 99, nil })
	b.ReportAllocs()
	for b.Loop() {
		n := 0
		for v, err := range pages {
			if err != nil {
				b.Fatal(err)
			}
			n += v
		}
		benchSink = n
	}
}
