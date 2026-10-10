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
