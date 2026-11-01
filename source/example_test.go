package source_test

import (
	"fmt"
	"io"
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
	// Output:
	// [ada cy]
	// [parse error on line 4, column 8: extraneous or missing " in quoted-field]
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
	})

	views, bad := events.
		Filter(func(e Event) bool { return e.Page == "/pricing" }).
		CollectAll()
	fmt.Println(views)
	fmt.Println(bad)
	// Output:
	// [{ada /pricing} {cy /pricing}]
	// [line 3: unexpected end of JSON input]
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
