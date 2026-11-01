---
title: "Sources"
description: "Ready-made producers for what pipelines most often start from — lines, CSV with or without a header, JSON Lines, pages and rows — in the catena/source and catena/source/sqlsource packages. Each takes a function that opens its resource rather than an open resource, and calls it inside the iteration: a pipeline that is built but never consumed opens nothing, and any exit — the end, an early stop, a panic — releases it. All of them return a Try. A failure of the source ends it; a failure of one record — a malformed row, a line that does not decode — is an element in its place, and the records after it still come. For anything not here, writing your own source is the same shape."
sidebar:
  order: 16
---

Ready-made producers for what pipelines most often start from — lines, CSV with or without a header, JSON Lines, pages and rows — in the `catena/source` and `catena/source/sqlsource` packages. Each takes a function that opens its resource rather than an open resource, and calls it inside the iteration: a pipeline that is built but never consumed opens nothing, and any exit — the end, an early stop, a panic — releases it. All of them return a `Try`. A failure of the source ends it; a failure of one record — a malformed row, a line that does not decode — is an element in its place, and the records after it still come. For anything not here, [writing your own source](/guides/writing-sources/) is the same shape.

## Lines

```go
func Lines(open func() (io.ReadCloser, error)) catena.Try[string]
```

Lines yields the lines of the reader that open returns, without their line endings ("\n" or "\r\n"); a final line with no ending is yielded too, and an empty input yields nothing. Lines may be of any length. open is called once per iteration, inside it, and the reader is closed on every exit; the error from Close is discarded. A failure to open, or to read, is yielded as ("", err) and ends the sequence. To skip a first line, settle the errors before Drop — UntilErr(&err).Drop(1) — since Drop counts an error as an element and would skip a failure to open as if it were the line.

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

CSV yields the records of the CSV data that open returns, as read by encoding/csv: each one a fresh slice of its fields. setup, if not nil, configures the reader before the first record — its separator, comment character, expected field count and so on — and ReuseRecord is turned off after it, since a lazy pipeline may hold on to a record. open is called once per iteration, inside it, and the reader is closed on every exit. A record that cannot be parsed — a stray quote, the wrong number of fields — is yielded as (nil, err), err a *csv.ParseError naming its line, and reading continues with the next record; a failure to open or to read ends the sequence. An opening quote that is never closed takes the rest of the input with it, as encoding/csv reads it, and is one error. For data with a header row, CSVHeader reads it for you: Drop(1) would also drop an error standing in its place — a file that failed to open — since Drop counts an error as an element.

:::caution
Holds one record in memory
:::

```go
// A price list with no header row, as some supplier systems still
// send it: SKU, then price in cents. One row was broken by hand.
prices := "kbd-01,8900\n" +
	"mse-07,35\"00\n" + // a stray quote: this row cannot be parsed
	"mon-27,32900\n"
rows := source.CSV(func() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(prices)), nil // os.Open(path) in real code
}, nil)

// The broken row is an element, not the end: CollectAll keeps every
// good row and says which line was wrong.
skus, problems := rows.Map(func(r []string) string { return r[0] }).CollectAll()
fmt.Println(skus)
fmt.Println(problems)
```

```
[kbd-01 mon-27]
[parse error on line 2, column 10: bare " in non-quoted-field]
```

## CSVHeader

```go
func CSVHeader(open func() (io.ReadCloser, error), setup func(*csv.Reader)) catena.Try[Record]
```

CSVHeader is CSV for data whose first record is a header: it reads the header, and yields every record after it as a Record, whose fields can be looked up by column name. A failure to open or to read, or a header that cannot be parsed, ends the sequence, as there is no way to name the columns; a malformed record after it is an element, and reading continues. A byte-order mark before the first column name, as spreadsheet exports write, is dropped. Data with no records at all yields nothing. Unless setup says otherwise, every record must have as many fields as the header.

:::caution
Holds the header and one record in memory
:::

```go
// A customer export from a spreadsheet: a header row names the
// columns, so the code reads fields by name and does not care what
// order the export put them in.
type Customer struct {
	Email string
	Seats int
}
export := "name,seats,email\n" +
	"Ada,12,ada@example.com\n" +
	"Bob,many,bob@example.com\n" + // seats is not a number
	"Cy,3,cy@example.com\n"
rows := source.CSVHeader(func() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(export)), nil
}, nil)

customers, problems := rows.MapErr(func(r source.Record) (Customer, error) {
	seats, err := strconv.Atoi(r.Get("seats"))
	if err != nil {
		return Customer{}, fmt.Errorf("%s: seats: %w", r.Get("email"), err)
	}
	return Customer{r.Get("email"), seats}, nil
}).CollectAll()
fmt.Println(customers)
fmt.Println(problems)
```

```
[{ada@example.com 12} {cy@example.com 3}]
[bob@example.com: seats: strconv.Atoi: parsing "many": invalid syntax]
```

## Get

```go
func (r Record) Get(column string) string
```

Get returns the field in the named column, or "" if there is none — which Lookup tells apart from an empty field. Where the header names a column twice, the first is meant.

```go
// Get is for the columns you know are there. A column the header does
// not have reads as "", like an empty field.
rows := source.CSVHeader(func() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("sku,qty\nkbd-01,3\n")), nil
}, nil)
for r, err := range rows {
	fmt.Printf("%s x%s, colour %q, err %v\n", r.Get("sku"), r.Get("qty"), r.Get("colour"), err)
}
```

```
kbd-01 x3, colour "", err <nil>
```

## Lookup

```go
func (r Record) Lookup(column string) (string, bool)
```

Lookup returns the field in the named column, and whether the header has such a column and this record a field for it.

```go
// Lookup tells a column that is missing from one that is empty — for
// exports whose optional columns come and go.
rows := source.CSVHeader(func() (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("sku,note\nkbd-01,\n")), nil
}, nil)
for r := range rows.Ignore() {
	note, hasNote := r.Lookup("note")
	colour, hasColour := r.Lookup("colour")
	fmt.Printf("note %q %v, colour %q %v\n", note, hasNote, colour, hasColour)
}
```

```
note "" true, colour "" false
```

## JSONLines

```go
func JSONLines[T any](open func() (io.ReadCloser, error), setup func(*json.Decoder)) catena.Try[T]
```

JSONLines yields the values of the JSON Lines data that open returns: each line decoded into a T by encoding/json. setup, if not nil, configures the decoder for every line — DisallowUnknownFields, so that a misspelt field is an error rather than a zero value, or UseNumber. Blank lines are skipped. A line that does not decode, or holds more than one value, is yielded as (zero, err), err wrapping json's error with the line's number ("line 3: …"), and reading continues; a failure to open or to read ends the sequence. open is called once per iteration, inside it, and the reader is closed on every exit.

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
}, nil)

views, bad := events.
	Filter(func(e Event) bool { return e.Page == "/pricing" }).
	CollectAll()
fmt.Println(views)
fmt.Println(bad)
```

```
[{ada /pricing} {cy /pricing}]
[line 3: JSON value cut short: unexpected EOF]
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
