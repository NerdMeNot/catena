package source_test

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/NerdMeNot/catena/source"
)

func ExampleLines() {
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
	// Output:
	// opened after building: 0
	// [GET /a GET /c]
	// opened after consuming: 1
}

func ExampleCSV() {
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
	// Output:
	// [kbd-01 mon-27]
	// [parse error on line 2, column 10: bare " in non-quoted-field]
}

func ExampleCSVHeader() {
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
	// Output:
	// [{ada@example.com 12} {cy@example.com 3}]
	// [bob@example.com: seats: strconv.Atoi: parsing "many": invalid syntax]
}

func ExampleRecord_Get() {
	// Get is for the columns you know are there. A column the header does
	// not have reads as "", like an empty field.
	rows := source.CSVHeader(func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader("sku,qty\nkbd-01,3\n")), nil
	}, nil)
	for r, err := range rows {
		fmt.Printf("%s x%s, colour %q, err %v\n", r.Get("sku"), r.Get("qty"), r.Get("colour"), err)
	}
	// Output: kbd-01 x3, colour "", err <nil>
}

func ExampleRecord_Lookup() {
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
	// Output: note "" true, colour "" false
}

func ExampleJSONLines() {
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
	// Output:
	// [{ada /pricing} {cy /pricing}]
	// [line 3: JSON value cut short: unexpected EOF]
}

func ExamplePaginate() {
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
	// Output: [ada bob cy] <nil> calls: 2
}
