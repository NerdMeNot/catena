// Package source holds ready-made producers for the things pipelines most
// often start from: the lines of a reader, CSV records with or without a
// header, JSON Lines, and
// the pages of a paginated API. catena/source/sqlsource has the
// database/sql one, kept apart so that importing this package does not
// link database/sql.
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
// Two kinds of failure are told apart. A failure of the source itself — it
// cannot be opened or read, a page cannot be fetched — ends the sequence,
// because there is no way on. A failure of one record — a malformed CSV
// row, a line that is not valid JSON — is an error element in that
// record's place, and the records after it still come: the consumer
// decides whether one bad record stops the run (Collect) or not
// (CollectAll, Ignore). sqlsource.Rows draws the same line, for a row that
// fails to scan.
//
// A producer is re-iterable exactly when calling its open function again
// is: each iteration opens afresh.
package source

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/NerdMeNot/catena"
)

// Lines yields the lines of the reader that open returns, without their
// line endings ("\n" or "\r\n"); a final line with no ending is yielded
// too, and an empty input yields nothing. Lines may be of any length. open
// is called once per iteration, inside it, and the reader is closed on
// every exit; the error from Close is discarded. A failure to open, or to
// read, is yielded as ("", err) and ends the sequence. To skip a first
// line, settle the errors before Drop — UntilErr(&err).Drop(1) — since
// Drop counts an error as an element and would skip a failure to open as
// if it were the line. ⚠ Holds one line, however long, in memory.
func Lines(open func() (io.ReadCloser, error)) catena.Try[string] {
	return func(yield func(string, error) bool) {
		rc, err := open()
		if err != nil {
			yield("", err)
			return
		}
		defer rc.Close()
		if err := eachLine(rc, func(line string) bool { return yield(line, nil) }); err != nil {
			yield("", err) // whatever was read of this line is dropped
		}
	}
}

// eachLine calls f with each line of r, without its line ending, until f
// returns false or r is exhausted. It returns the error that stopped the
// reading, or nil at the end of r or when f stopped it.
func eachLine(r io.Reader, f func(line string) bool) error {
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return err
		}
		if err == nil || line != "" {
			// Only a CR that is part of a line ending goes: a last line
			// cut short after its CR keeps it.
			if body, ended := strings.CutSuffix(line, "\n"); ended {
				line = strings.TrimSuffix(body, "\r")
			}
			if !f(line) {
				return nil
			}
		}
		if err != nil {
			return nil // io.EOF
		}
	}
}

// CSV yields the records of the CSV data that open returns, as read by
// encoding/csv: each one a fresh slice of its fields. setup, if not nil,
// configures the reader before the first record — its separator, comment
// character, expected field count and so on — and ReuseRecord is turned
// off after it, since a lazy pipeline may hold on to a record. open is
// called once per iteration, inside it, and the reader is closed on every
// exit. A record that cannot be parsed — a stray quote, the wrong number of
// fields — is yielded as (nil, err), err a *csv.ParseError naming its line,
// and reading continues with the next record; a failure to open or to
// read ends the sequence. An opening quote that is never closed takes the
// rest of the input with it, as encoding/csv reads it, and is one error.
// For data with a header row, CSVHeader reads it for you: Drop(1) would
// also drop an error standing in its place — a file that failed to open —
// since Drop counts an error as an element. ⚠ Holds one record in memory.
func CSV(open func() (io.ReadCloser, error), setup func(*csv.Reader)) catena.Try[[]string] {
	return func(yield func([]string, error) bool) {
		rc, err := open()
		if err != nil {
			yield(nil, err)
			return
		}
		defer rc.Close()
		r := csv.NewReader(rc)
		if setup != nil {
			setup(r)
		}
		r.ReuseRecord = false
		for {
			rec, err := r.Read()
			if err == io.EOF {
				return
			}
			var perr *csv.ParseError
			if err != nil && !errors.As(err, &perr) {
				yield(nil, err)
				return
			}
			if err != nil {
				rec = nil // a short record arrives with its error
			}
			if !yield(rec, err) {
				return
			}
		}
	}
}

// CSVHeader is CSV for data whose first record is a header: it reads the
// header, and yields every record after it as a Record, whose fields can be
// looked up by column name. A failure to open or to read, or a header that
// cannot be parsed, ends the sequence, as there is no way to name the
// columns; a malformed record after it is an element, and reading
// continues. A byte-order mark before the first column name, as
// spreadsheet exports write, is dropped. Data with no records at all
// yields nothing. Unless setup says otherwise, every record must have as
// many fields as the header. ⚠ Holds the header and one record in memory.
func CSVHeader(open func() (io.ReadCloser, error), setup func(*csv.Reader)) catena.Try[Record] {
	records := CSV(open, setup)
	return func(yield func(Record, error) bool) {
		var columns map[string]int // nil until the header is read
		records(func(fields []string, err error) bool {
			switch {
			case columns == nil && err != nil:
				yield(Record{}, err)
				return false
			case columns == nil:
				columns = make(map[string]int, len(fields))
				for i, name := range fields {
					if i == 0 {
						name = strings.TrimPrefix(name, "\ufeff")
					}
					if _, dup := columns[name]; !dup {
						columns[name] = i
					}
				}
				return true
			case err != nil:
				return yield(Record{}, err)
			}
			return yield(Record{Fields: fields, columns: columns}, nil)
		})
	}
}

// Record is one record of CSV data read by CSVHeader: its fields, in
// order, and the header's column names to find them by.
type Record struct {
	Fields  []string
	columns map[string]int // shared by the records of one iteration
}

// Get returns the field in the named column, or "" if there is none —
// which Lookup tells apart from an empty field. Where the header names a
// column twice, the first is meant.
func (r Record) Get(column string) string {
	v, _ := r.Lookup(column)
	return v
}

// Lookup returns the field in the named column, and whether the header
// has such a column and this record a field for it.
func (r Record) Lookup(column string) (string, bool) {
	i, ok := r.columns[column]
	if !ok || i >= len(r.Fields) {
		return "", false
	}
	return r.Fields[i], true
}

// JSONLines yields the values of the JSON Lines data that open returns:
// each line decoded into a T by encoding/json. setup, if not nil,
// configures the decoder for every line — DisallowUnknownFields, so that a
// misspelt field is an error rather than a zero value, or UseNumber.
// Blank lines are skipped. A line that does not decode, or holds more than
// one value, is yielded as (zero, err), err wrapping json's error with the
// line's number ("line 3: …"), and reading continues; a failure to open or
// to read ends the sequence. open is called once per iteration, inside it,
// and the reader is closed on every exit. ⚠ Holds one line, however long,
// in memory.
func JSONLines[T any](open func() (io.ReadCloser, error), setup func(*json.Decoder)) catena.Try[T] {
	return func(yield func(T, error) bool) {
		var zero T
		rc, err := open()
		if err != nil {
			yield(zero, err)
			return
		}
		defer rc.Close()
		n := 0
		err = eachLine(rc, func(line string) bool {
			n++
			if strings.TrimSpace(line) == "" {
				return true
			}
			v, err := decodeLine[T](line, setup)
			if err != nil {
				return yield(zero, fmt.Errorf("line %d: %w", n, err))
			}
			return yield(v, nil)
		})
		if err != nil {
			yield(zero, err)
		}
	}
}

// decodeLine decodes the one JSON value a line must hold.
func decodeLine[T any](line string, setup func(*json.Decoder)) (T, error) {
	var v T
	dec := json.NewDecoder(strings.NewReader(line))
	if setup != nil {
		setup(dec)
	}
	if err := dec.Decode(&v); err == io.ErrUnexpectedEOF {
		return v, fmt.Errorf("JSON value cut short: %w", err)
	} else if err != nil {
		return v, err
	}
	var extra json.RawMessage
	if err := dec.Decode(&extra); err != io.EOF {
		return v, errMoreThanOne
	}
	return v, nil
}

var errMoreThanOne = errors.New("more than one JSON value on the line")

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
