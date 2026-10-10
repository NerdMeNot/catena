package catena_test

// Runnable examples: lazy acquisition, the pattern behind every producer in
// catena/source — the resource is opened inside the iteration, so an
// unconsumed pipeline holds nothing and early termination releases it.

import (
	"fmt"
	"io"
	"strings"

	"github.com/NerdMeNot/catena/source"
)

func Example_lazyAcquisition() {
	opened := 0
	logs := source.Lines(func() (io.ReadCloser, error) {
		opened++
		return io.NopCloser(strings.NewReader("GET /a\nPOST /b\nGET /c\nGET /d")), nil
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
