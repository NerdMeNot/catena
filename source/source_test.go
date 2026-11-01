package source_test

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/source"
)

var errBoom = errors.New("boom")

// tracker is a reader that counts how often it was opened and closed.
type tracker struct {
	opens, closes int
	body          func() io.Reader
}

type closer struct {
	io.Reader
	t *tracker
}

func (c closer) Close() error { c.t.closes++; return errors.New("close errors are discarded") }

func (t *tracker) open() (io.ReadCloser, error) {
	t.opens++
	return closer{t.body(), t}, nil
}

func text(s string) *tracker {
	return &tracker{body: func() io.Reader { return strings.NewReader(s) }}
}

func TestLinesSplitting(t *testing.T) {
	long := strings.Repeat("x", 200_000) // far past bufio.Scanner's 64 KB limit
	for in, want := range map[string][]string{
		"":             nil,
		"a":            {"a"},
		"a\n":          {"a"},
		"a\nb":         {"a", "b"},
		"a\r\nb\r\n":   {"a", "b"},
		"a\n\nb\n":     {"a", "", "b"},
		"\n":           {""},
		"a\rb\n":       {"a\rb"},     // a lone CR is not a line ending
		"a\nb\r":       {"a", "b\r"}, // nor is one at the very end
		"a\r\n\r":      {"a", "\r"},
		long + "\nend": {long, "end"},
	} {
		tr := text(in)
		got, err := source.Lines(tr.open).Collect()
		if err != nil || !slices.Equal(got, want) {
			t.Errorf("%.20q: got %.40q, %v; want %.40q", in, got, err, want)
		}
		if tr.opens != 1 || tr.closes != 1 {
			t.Errorf("%.20q: opens=%d closes=%d", in, tr.opens, tr.closes)
		}
	}
}

// lineSources are the producers that read a stream line by line or
// record by record, each flattened to Try[string]. Every one gets the same
// lifecycle checks; the fixture text is valid input for all three.
var lineSources = []struct {
	name string
	of   func(open func() (io.ReadCloser, error)) catena.Try[string]
}{
	{"Lines", source.Lines},
	{"CSV", func(open func() (io.ReadCloser, error)) catena.Try[string] {
		return source.CSV(open, nil).Map(func(rec []string) string { return strings.Join(rec, ",") })
	}},
	{"JSONLines", source.JSONLines[string]},
}

func TestLineSourcesLifecycle(t *testing.T) {
	const three = "\"a\"\n\"b\"\n\"c\"\n"
	for _, src := range lineSources {
		t.Run(src.name, func(t *testing.T) {
			t.Run("never consumed, never opened", func(t *testing.T) {
				tr := text(three)
				_ = src.of(tr.open).Ignore().Take(1)
				if tr.opens != 0 {
					t.Fatalf("opened %d times by building a pipeline", tr.opens)
				}
			})

			t.Run("early termination closes", func(t *testing.T) {
				tr := text(three)
				if got := src.of(tr.open).Ignore().Take(2).Collect(); len(got) != 2 {
					t.Fatalf("got %v", got)
				}
				if tr.opens != 1 || tr.closes != 1 {
					t.Fatalf("opens=%d closes=%d", tr.opens, tr.closes)
				}
			})

			t.Run("a panic downstream closes", func(t *testing.T) {
				tr := text(three)
				func() {
					defer func() { _ = recover() }()
					src.of(tr.open).ForEach(func(string) error { panic("downstream") })
				}()
				if tr.closes != 1 {
					t.Fatalf("closes=%d", tr.closes)
				}
			})

			t.Run("each iteration opens afresh", func(t *testing.T) {
				tr := text(three)
				seq := src.of(tr.open)
				first, _ := seq.Collect()
				second, _ := seq.Collect()
				if len(first) != 3 || !slices.Equal(first, second) || tr.opens != 2 || tr.closes != 2 {
					t.Fatalf("%v then %v, opens=%d closes=%d", first, second, tr.opens, tr.closes)
				}
			})

			t.Run("open failure is the only element", func(t *testing.T) {
				vals, errs := src.of(func() (io.ReadCloser, error) { return nil, errBoom }).CollectAll()
				if vals != nil || len(errs) != 1 || !errors.Is(errs[0], errBoom) {
					t.Fatalf("vals=%v errs=%v", vals, errs)
				}
			})

			t.Run("read failure ends the sequence and drops the partial line", func(t *testing.T) {
				tr := &tracker{body: func() io.Reader {
					return io.MultiReader(strings.NewReader("\"a\"\n\"parti"), iotest.ErrReader(errBoom))
				}}
				var got []string
				var errs []error
				for v, err := range src.of(tr.open) {
					if err != nil {
						errs = append(errs, err)
						if v != "" {
							t.Fatalf("value %q alongside an error", v)
						}
						continue
					}
					got = append(got, v)
				}
				if len(got) != 1 || len(errs) != 1 || !errors.Is(errs[0], errBoom) || tr.closes != 1 {
					t.Fatalf("got=%v errs=%v closes=%d", got, errs, tr.closes)
				}
			})
		})
	}
}

func TestCSV(t *testing.T) {
	read := func(in string, setup func(*csv.Reader)) ([][]string, []error) {
		var recs [][]string
		var errs []error
		for rec, err := range source.CSV(text(in).open, setup) {
			if err != nil {
				if rec != nil {
					t.Fatalf("%q: record %q alongside an error", in, rec)
				}
				errs = append(errs, err)
				continue
			}
			recs = append(recs, rec)
		}
		return recs, errs
	}
	parseErr := func(err error) bool {
		var pe *csv.ParseError
		return errors.As(err, &pe)
	}

	t.Run("records, quoting and line endings", func(t *testing.T) {
		recs, errs := read("id,note\r\n1,\"two\nlines\"\r\n2,\"say \"\"hi\"\"\"\n", nil)
		want := [][]string{{"id", "note"}, {"1", "two\nlines"}, {"2", `say "hi"`}}
		if errs != nil || !slices.EqualFunc(recs, want, slices.Equal) {
			t.Fatalf("got %q %v", recs, errs)
		}
	})

	t.Run("a bad record is an element, and reading goes on", func(t *testing.T) {
		recs, errs := read("a,b\nx\"y,z\nc,d\n", nil)
		if len(recs) != 2 || len(errs) != 1 || !parseErr(errs[0]) {
			t.Fatalf("got %q %v", recs, errs)
		}
	})

	t.Run("the wrong number of fields is a bad record", func(t *testing.T) {
		recs, errs := read("a,b\nx,y,z\nc,d\n", nil) // the first record sets the count
		if len(recs) != 2 || len(errs) != 1 || !errors.Is(errs[0], csv.ErrFieldCount) {
			t.Fatalf("got %q %v", recs, errs)
		}
	})

	t.Run("an unclosed quote takes the rest, as one error", func(t *testing.T) {
		recs, errs := read("a,b\nc,\"d\ne,f\n", nil)
		if len(recs) != 1 || len(errs) != 1 || !parseErr(errs[0]) {
			t.Fatalf("got %q %v", recs, errs)
		}
	})

	t.Run("setup configures the reader", func(t *testing.T) {
		recs, errs := read("# exported\na;b\nc;d\n", func(r *csv.Reader) {
			r.Comma = ';'
			r.Comment = '#'
		})
		if errs != nil || !slices.EqualFunc(recs, [][]string{{"a", "b"}, {"c", "d"}}, slices.Equal) {
			t.Fatalf("got %q %v", recs, errs)
		}
	})

	t.Run("records are never reused", func(t *testing.T) {
		recs, _ := read("a,b\nc,d\n", func(r *csv.Reader) { r.ReuseRecord = true })
		if recs[0][0] != "a" || recs[1][0] != "c" {
			t.Fatalf("an earlier record changed under a later one: %q", recs)
		}
	})

	t.Run("a reader that cannot work ends the sequence", func(t *testing.T) {
		// Not a record's fault but the configuration's: no way on.
		recs, errs := read("a,b\nc,d\n", func(r *csv.Reader) { r.Comma = '"' })
		if recs != nil || len(errs) != 1 || parseErr(errs[0]) {
			t.Fatalf("got %q %v", recs, errs)
		}
	})
}

func TestJSONLines(t *testing.T) {
	type event struct {
		Kind string `json:"kind"`
		N    int    `json:"n"`
	}
	in := "{\"kind\":\"click\",\"n\":1}\r\n\n  \n{\"kind\":\"view\",\"n\":2}\nnot json\n{\"kind\":\"click\",\"n\":\"three\"}\n{\"kind\":\"buy\",\"n\":4}"
	events, errs := source.JSONLines[event](text(in).open).CollectAll()
	want := []event{{"click", 1}, {"view", 2}, {"buy", 4}}
	if !slices.Equal(events, want) || len(errs) != 2 {
		t.Fatalf("got %v %v", events, errs)
	}
	// Errors name the line, counting the blank ones, and keep json's error.
	var syntax *json.SyntaxError
	var typ *json.UnmarshalTypeError
	if !strings.HasPrefix(errs[0].Error(), "line 5: ") || !errors.As(errs[0], &syntax) ||
		!strings.HasPrefix(errs[1].Error(), "line 6: ") || !errors.As(errs[1], &typ) {
		t.Fatalf("errors: %v", errs)
	}
}

// pages is a paginated source of the integers below total, size at a time,
// recording every cursor it was asked for.
type pages struct {
	total, size int
	failAt      int // cursor at which fetch fails, or -1
	asked       []int
}

func (p *pages) fetch(cursor int) ([]int, int, bool, error) {
	p.asked = append(p.asked, cursor)
	if cursor == p.failAt {
		return []int{-1, -2}, cursor, true, errBoom // items alongside an error must be ignored
	}
	end := min(cursor+p.size, p.total)
	var items []int
	for i := cursor; i < end; i++ {
		items = append(items, i)
	}
	return items, end, end < p.total, nil
}

func TestPaginate(t *testing.T) {
	t.Run("yields every page in order", func(t *testing.T) {
		p := &pages{total: 7, size: 3, failAt: -1}
		got, err := source.Paginate(0, p.fetch).Collect()
		if err != nil || !slices.Equal(got, []int{0, 1, 2, 3, 4, 5, 6}) || !slices.Equal(p.asked, []int{0, 3, 6}) {
			t.Fatalf("got %v, %v; asked %v", got, err, p.asked)
		}
	})

	t.Run("never consumed, never fetched", func(t *testing.T) {
		p := &pages{total: 7, size: 3, failAt: -1}
		_ = source.Paginate(0, p.fetch).Ignore().Take(1)
		if p.asked != nil {
			t.Fatalf("fetched %v by building a pipeline", p.asked)
		}
	})

	t.Run("fetches no page it does not need", func(t *testing.T) {
		p := &pages{total: 100, size: 3, failAt: -1}
		// The third element is the last of the first page: the second
		// page must not be fetched to deliver it.
		if got := source.Paginate(0, p.fetch).Ignore().Take(3).Collect(); !slices.Equal(got, []int{0, 1, 2}) {
			t.Fatalf("got %v", got)
		}
		if !slices.Equal(p.asked, []int{0}) {
			t.Fatalf("asked for %v", p.asked)
		}
	})

	t.Run("empty pages and an empty source", func(t *testing.T) {
		calls := 0
		sparse := source.Paginate("a", func(cursor string) ([]int, string, bool, error) {
			calls++
			switch cursor {
			case "a":
				return nil, "b", true, nil // an empty page is not the end
			case "b":
				return []int{1}, "c", true, nil
			}
			return nil, "", false, nil
		})
		if got, err := sparse.Collect(); err != nil || !slices.Equal(got, []int{1}) || calls != 3 {
			t.Fatalf("got %v, %v after %d calls", got, err, calls)
		}
	})

	t.Run("a failed fetch ends the sequence", func(t *testing.T) {
		p := &pages{total: 9, size: 3, failAt: 3}
		vals, errs := source.Paginate(0, p.fetch).CollectAll()
		if !slices.Equal(vals, []int{0, 1, 2}) || len(errs) != 1 || !errors.Is(errs[0], errBoom) {
			t.Fatalf("vals=%v errs=%v", vals, errs)
		}
		if !slices.Equal(p.asked, []int{0, 3}) {
			t.Fatalf("asked for %v after the failure", p.asked)
		}
	})

	t.Run("each iteration starts from the first cursor", func(t *testing.T) {
		p := &pages{total: 4, size: 2, failAt: -1}
		seq := source.Paginate(0, p.fetch)
		first, _ := seq.Collect()
		second, _ := seq.Collect()
		if !slices.Equal(first, second) || !slices.Equal(p.asked, []int{0, 2, 0, 2}) {
			t.Fatalf("%v then %v, asked %v", first, second, p.asked)
		}
	})

	t.Run("an endless source is safe behind Take", func(t *testing.T) {
		endless := source.Paginate(0, func(c int) ([]int, int, bool, error) { return []int{c}, c + 1, true, nil })
		if got := endless.Ignore().Take(4).Collect(); !slices.Equal(got, []int{0, 1, 2, 3}) {
			t.Fatalf("got %v", got)
		}
	})
}

var _ catena.Try[string] = source.Lines(nil) // the producers are plain Try values
