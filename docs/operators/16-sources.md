# Sources

Ready-made producers for what pipelines most often start from — lines, CSV, JSON Lines, pages and rows — in the `catena/source` and `catena/source/sqlsource` packages. Each takes a function that opens its resource rather than an open resource, and calls it inside the iteration: a pipeline that is built but never consumed opens nothing, and any exit — the end, an early stop, a panic — releases it. All of them return a `Try`. A failure of the source ends it; a failure of one record — a malformed row, a line that does not decode — is an element in its place, and the records after it still come. For anything not here, [writing your own source](../09-writing-sources.md) is the same shape.

## Lines

```go
func Lines(open func() (io.ReadCloser, error)) catena.Try[string]
```

Lines yields the lines of the reader that open returns, without their line endings ("\n" or "\r\n"); a final line with no ending is yielded too, and an empty input yields nothing. Lines may be of any length. open is called once per iteration, inside it, and the reader is closed on every exit; the error from Close is discarded. A failure to open, or to read, is yielded as ("", err) and ends the sequence.

:::caution
Holds one line, however long, in memory
:::

```go
opened := 0
logs := source.Lines(func() (io.ReadCloser, error) {
	opened++ // in real code: os.Open(path)
	return io.NopCloser(strings.NewReader("GET /a\nPOST /b\nGET /c\nGET /d\n")), nil
})

// Building the pipeline opens nothing.
gets := logs.Ignore().
	Filter(func(l string) bool { return strings.HasPrefix(l, "GET ") }).
	Take(2)
fmt.Println("opened after building:", opened)

fmt.Println(gets.Collect())
fmt.Println("opened after consuming:", opened)
```

```
opened after building: 0
[GET /a GET /c]
opened after consuming: 1
```

## CSV

```go
func CSV(open func() (io.ReadCloser, error), setup func(*csv.Reader)) catena.Try[[]string]
```

CSV yields the records of the CSV data that open returns, as read by encoding/csv: each one a fresh slice of its fields. setup, if not nil, configures the reader before the first record — its separator, comment character, expected field count and so on — and ReuseRecord is turned off after it, since a lazy pipeline may hold on to a record. open is called once per iteration, inside it, and the reader is closed on every exit. A record that cannot be parsed — a stray quote, the wrong number of fields — is yielded as (nil, err), err a *csv.ParseError naming its line, and reading continues with the next record; a failure to open or to read ends the sequence. An opening quote that is never closed takes the rest of the input with it, as encoding/csv reads it, and is one error. A header row is a record like any other: Drop(1) skips it.

:::caution
Holds one record in memory
:::

```go
// A customer export, with a header, a row someone broke by hand, and a
// note that runs over two lines inside its quotes.
export := "id,name,note\n" +
	"1,ada,\"prefers email,\nnot phone\"\n" +
	"2,bob,\"\"vip\"\n" + // a stray quote: this row cannot be parsed
	"3,cy,\n"
rows := source.CSV(func() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(export)), nil // os.Open(path) in real code
}, nil)

// The broken row is an element, not the end: CollectAll keeps every
// good row and says which line was wrong.
names, problems := rows.Drop(1). // the header
					Map(func(r []string) string { return r[1] }).
					CollectAll()
fmt.Println(names)
fmt.Println(problems)
```

```
[ada cy]
[parse error on line 4, column 8: extraneous or missing " in quoted-field]
```

## JSONLines

```go
func JSONLines[T any](open func() (io.ReadCloser, error)) catena.Try[T]
```

JSONLines yields the values of the JSON Lines data that open returns: each line decoded into a T by encoding/json. Blank lines are skipped. A line that does not decode is yielded as (zero, err), err wrapping json's error with the line's number ("line 3: …"), and reading continues; a failure to open or to read ends the sequence. open is called once per iteration, inside it, and the reader is closed on every exit.

:::caution
Holds one line, however long, in memory
:::

```go
// Click events, one JSON object per line, as most log shippers and
// data exports write them. One line was cut off mid-write.
type Event struct {
	User string `json:"user"`
	Page string `json:"page"`
}
log := `{"user":"ada","page":"/pricing"}
{"user":"bob","page":"/docs"}
{"user":"ada","pa
{"user":"cy","page":"/pricing"}
`
events := source.JSONLines[Event](func() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(log)), nil
})

views, bad := events.
	Filter(func(e Event) bool { return e.Page == "/pricing" }).
	CollectAll()
fmt.Println(views)
fmt.Println(bad)
```

```
[{ada /pricing} {cy /pricing}]
[line 3: unexpected end of JSON input]
```

## Paginate

```go
func Paginate[T, C any](first C, fetch func(cursor C) (items []T, next C, more bool, err error)) catena.Try[T]
```

Paginate yields the items of a paginated source, one page at a time. fetch is given a cursor — first, to begin with — and returns that page's items, the cursor of the next page, and whether there is one. A page is fetched only when the one before it has been yielded in full, so a downstream Take(n) fetches no more pages than it needs. A failed fetch is yielded as (zero, err) and ends the sequence, its items ignored: the way forward is not known. Each iteration starts again from first.

:::caution
- Holds one page in memory
- A source that always reports another page is infinite, and one that reports empty pages without end never yields, so nothing downstream — not even Take — can stop it: bound that in fetch
:::

```go
// A paginated API: each call returns a page and the token for the
// next one. Here it is three pages of two users.
users := []string{"ada", "bob", "cy", "dee", "eve", "flo"}
calls := 0
listUsers := func(offset int) (page []string, next int, more bool, err error) {
	calls++
	end := min(offset+2, len(users))
	return users[offset:end], end, end < len(users), nil
}

// Three users are on the first two pages, so the third is not fetched.
first3, err := source.Paginate(0, listUsers).Take(3).Collect()
fmt.Println(first3, err, "calls:", calls)
```

```
[ada bob cy] <nil> calls: 2
```

## Rows

```go
func Rows[T any](open func() (*sql.Rows, error), scan func(*sql.Rows) (T, error)) catena.Try[T]
```

Rows yields one element per row of the result set that open returns, each produced by scan. open is called once per iteration, inside it, so a pipeline that is built but never consumed runs no query; the rows are closed on every exit, including a downstream stage stopping early. A failure to open is yielded as (zero, err) and ends the sequence. A row that fails to scan is yielded as (zero, err) and iteration continues — the consumer picks the policy. An error that ends the result set early (rows.Err) is yielded last. scan must not call rows.Next or rows.Close.

:::caution
Holds the connection behind the rows until the iteration returns
:::

```go
db, _ := sql.Open("sqlsource-mem", "")
defer db.Close()

type order struct{ ID int }
orders := sqlsource.Rows(
	func() (*sql.Rows, error) { return db.Query("4") }, // in real code: your SELECT
	func(rows *sql.Rows) (order, error) {
		var o order
		err := rows.Scan(&o.ID)
		return o, err
	},
)

// Nothing has been queried yet. Take(2) stops early, and the rows are
// closed before Collect returns.
first2, err := orders.Take(2).Collect()
fmt.Println(first2, err)
```

```
[{0} {1}] <nil>
```
