package catena_test

// Runnable examples: the resource-owning producer patterns the spec
// commits to documenting (§4.2 lazy acquisition, §7.7 reader adapter),
// plus the operators that justify the library.

import (
	"bufio"
	"database/sql"
	"fmt"
	"strings"

	"github.com/NerdMeNot/catena"
)

// Lines adapts an io.Reader into a Try of its lines — the §7.7 pattern.
// The reader is wrapped lazily inside the closure: a sequence that is
// never consumed never reads, and early termination stops the scan.
func Lines(open func() (*strings.Reader, error)) catena.Try[string] {
	return func(yield func(string, error) bool) {
		r, err := open()
		if err != nil {
			yield("", err)
			return
		}
		sc := bufio.NewScanner(r)
		for sc.Scan() {
			if !yield(sc.Text(), nil) {
				return
			}
		}
		if err := sc.Err(); err != nil {
			yield("", err)
		}
	}
}

func Example_lazyAcquisition() {
	opened := 0
	logs := Lines(func() (*strings.Reader, error) {
		opened++
		return strings.NewReader("GET /a\nPOST /b\nGET /c\nGET /d"), nil
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

// Rows is the §4.2 producer pattern for *sql.Rows: lazy acquisition, so
// an unconsumed Try holds no resource, and defer runs on early
// termination through any number of stages. bake_test.go exercises it
// against a real database/sql driver.
func Rows[T any](open func() (*sql.Rows, error), scan func(*sql.Rows) (T, error)) catena.Try[T] {
	return func(yield func(T, error) bool) {
		var zero T
		rows, err := open()
		if err != nil {
			yield(zero, err)
			return
		}
		defer rows.Close()
		for rows.Next() {
			v, err := scan(rows)
			if err != nil {
				v = zero
			}
			if !yield(v, err) {
				return
			}
		}
		if err := rows.Err(); err != nil {
			yield(zero, err)
		}
	}
}
