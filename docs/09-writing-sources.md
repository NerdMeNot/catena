# Writing your own source

catena ships sources for the common starting points — `source.Lines`,
`source.CSV`, `source.CSVHeader`, `source.JSONLines`, `source.Paginate`
and `sqlsource.Rows` —
and stops there on purpose: a source for Kafka, S3 or your company's
internal API would bring a dependency with it, and the shape is short
enough to write. This guide is that shape, the rules that make a source
behave like the shipped ones, and how to test that it does.

## The shape

A source is a function that returns a `Try`: a closure that, when it is
iterated, opens what it needs, yields each element or error, and cleans
up on the way out. [Error handling](03-error-handling.md#writing-a-producer-that-owns-a-resource)
walks through it for a database query, which is exactly how
`sqlsource.Rows` is written. The same shape wraps any client with a
`Next`/`Err`/`Close` style of API.

Many APIs are the other way round: you hand them a callback and they call
it. `fs.WalkDir` is one, and catena ships no source for it, so here it is:

```go
type File struct {
    Path string
    Size int64
}

// Files yields every regular file under root, in lexical order. A
// directory that cannot be read is an error element and the walk goes on
// past it; a root that cannot be read is the only element.
func Files(fsys fs.FS, root string) catena.Try[File] {
    return func(yield func(File, error) bool) {
        fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
            if err != nil {
                if !yield(File{}, err) {
                    return fs.SkipAll
                }
                return nil // carry on past it
            }
            if !d.Type().IsRegular() {
                return nil
            }
            info, err := d.Info()
            if err != nil {
                if !yield(File{}, err) {
                    return fs.SkipAll
                }
                return nil
            }
            if !yield(File{path, info.Size()}, nil) {
                return fs.SkipAll // the consumer stopped: so does the walk
            }
            return nil
        })
    }
}
```

It composes like anything else, and stopping early stops the walk rather
than finishing it unseen:

```go
big, err := Files(os.DirFS("dist"), ".").
    Filter(func(f File) bool { return f.Size > 100_000 }).
    Collect()
```

The trick for any callback API is the same: when `yield` returns false,
tell the API to stop — `fs.SkipAll` here, a sentinel error or a cancelled
context elsewhere — and return.

## The rules

These are what make a source behave like the shipped ones. Each is one
line of code, and each is easy to forget.

- **Open inside the closure.** Take an opener, or build the request
  inside, so that a pipeline which is built but never consumed opens
  nothing — and so that iterating twice opens twice, which makes the
  source re-iterable.
- **Release with `defer`.** It runs when the sequence ends, when a
  downstream `Take` or `First` stops it early, and when a callback
  downstream panics. There is no `Close` on an iterator; `defer` is the
  close.
- **Stop the moment `yield` returns false.** Calling it again panics under
  a `range` loop, and is a bug everywhere.
  Return, or tell the API you are wrapping to stop.
- **A failure to open is the only element.** Yield it and return.
- **Tell the two kinds of failure apart.** If the source itself fails —
  it cannot be read, the connection dropped, a page cannot be fetched —
  yield the error and end, since there is no way on. If one record is bad
  — it will not parse, decode or scan — yield the error in its place and
  carry on. The consumer then chooses: `Collect` stops at either,
  `CollectAll` keeps the good records and lists the bad ones, and
  `UntilErr` before anything positional stops at the first.
- **Yield the zero value with an error.** Never a half-built value
  alongside one.
- **Check the trailing error.** `rows.Err()`, `scanner.Err()`, a decoder's
  last error: an API that reports failure after the loop has to be asked,
  or the failure looks like a shorter result. Go 1.28's vet is set to
  flag this for `sql.Rows` and `bufio.Scanner`.

## Testing a source

The two properties that matter can be tested without a real resource,
with a reader that counts how often it was opened and closed:

```go
type tracker struct{ opens, closes int }

type counted struct {
    io.Reader
    t *tracker
}

func (c counted) Close() error { c.t.closes++; return nil }

func (t *tracker) open() (io.ReadCloser, error) {
    t.opens++
    return counted{strings.NewReader("a\nb\nc\n"), t}, nil
}

func TestNeverConsumedNeverOpened(t *testing.T) {
    var tr tracker
    _ = MySource(tr.open).Ignore().Take(1) // built, not run
    if tr.opens != 0 {
        t.Fatalf("opened %d times by building a pipeline", tr.opens)
    }
}

func TestEarlyStopCloses(t *testing.T) {
    var tr tracker
    MySource(tr.open).Ignore().Take(1).Collect()
    if tr.opens != 1 || tr.closes != 1 {
        t.Fatalf("opens=%d closes=%d", tr.opens, tr.closes)
    }
}
```

Add the same check with a downstream panic (recover it, then count the
closes), with an opener that fails (one element, the error), and with a
reader that fails part-way (the error ends the sequence). That is the
whole list the shipped sources are tested against, in
`source/source_test.go`.
