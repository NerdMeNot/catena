// Package source holds ready-made producers for the things pipelines most
// often start from: the lines of a reader, and the pages of a paginated
// API. catena/source/sqlsource has the database/sql one, kept apart so
// that importing this package does not link database/sql.
//
// They are all the same pattern — lazy acquisition, described in the
// error-handling guide — written once and tested:
//
//   - Never consumed, never opened. A producer takes a function that opens
//     its resource, not an open resource, and calls it inside the
//     iteration: a pipeline that is built but not run holds nothing.
//   - Any exit releases. The resource is closed when the sequence ends,
//     when a downstream stage stops early, and when a callback panics.
//   - Failure is an element. Every producer returns a catena.Try, and a
//     failure to open is its first and only element.
//
// A producer is re-iterable exactly when calling its open function again
// is: each iteration opens afresh.
package source

import (
	"bufio"
	"io"
	"strings"

	"github.com/NerdMeNot/catena"
)

// Lines yields the lines of the reader that open returns, without their
// line endings ("\n" or "\r\n"); a final line with no ending is yielded
// too, and an empty input yields nothing. Lines may be of any length. open
// is called once per iteration, inside it, and the reader is closed on
// every exit; the error from Close is discarded. A failure to open, or to
// read, is yielded as ("", err) and ends the sequence. ⚠ Holds one line,
// however long, in memory.
func Lines(open func() (io.ReadCloser, error)) catena.Try[string] {
	return func(yield func(string, error) bool) {
		rc, err := open()
		if err != nil {
			yield("", err)
			return
		}
		defer rc.Close()
		r := bufio.NewReader(rc)
		for {
			line, err := r.ReadString('\n')
			if err != nil && err != io.EOF {
				yield("", err) // whatever was read of this line is dropped
				return
			}
			if err == nil || line != "" {
				// Only a CR that is part of a line ending goes: a
				// last line cut short after its CR keeps it.
				if body, ended := strings.CutSuffix(line, "\n"); ended {
					line = strings.TrimSuffix(body, "\r")
				}
				if !yield(line, nil) {
					return
				}
			}
			if err != nil {
				return // io.EOF
			}
		}
	}
}

// Paginate yields the items of a paginated source, one page at a time.
// fetch is given a cursor — first, to begin with — and returns that page's
// items, the cursor of the next page, and whether there is one. A page is
// fetched only when the one before it has been yielded in full, so a
// downstream Take(n) fetches no more pages than it needs. A failed fetch
// is yielded as (zero, err) and ends the sequence, its items ignored: the
// way forward is not known. Each iteration starts again from first. ⚠
// Holds one page in memory. ⚠ A source that always reports another page
// is infinite, and one that reports empty pages without end never yields,
// so nothing downstream — not even Take — can stop it: bound that in
// fetch.
func Paginate[T, C any](first C, fetch func(cursor C) (items []T, next C, more bool, err error)) catena.Try[T] {
	return func(yield func(T, error) bool) {
		cursor := first
		for {
			items, next, more, err := fetch(cursor)
			if err != nil {
				var zero T
				yield(zero, err)
				return
			}
			for _, v := range items {
				if !yield(v, nil) {
					return
				}
			}
			if !more {
				return
			}
			cursor = next
		}
	}
}
