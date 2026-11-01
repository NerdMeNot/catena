// Sources: the producers in catena/source open what they read inside the
// iteration, so a pipeline that is built holds nothing, and a pipeline
// that stops early releases what it opened. Lines reads a file; CSV and
// JSONLines read records, where one bad record is an element rather than
// the end; Paginate walks a paginated API and fetches no page it does not
// need; and all of them feed a concurrent stage like any other sequence.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/NerdMeNot/catena/parallel"
	"github.com/NerdMeNot/catena/source"
)

type User struct {
	ID   int
	Name string
}

// api stands in for a paginated HTTP API: page size two, an integer
// offset for a cursor, and a counter so the example can show how many
// requests a pipeline really made.
type api struct {
	users    []User
	requests int
}

func (a *api) list(offset int) (page []User, next int, more bool, err error) {
	a.requests++
	end := min(offset+2, len(a.users))
	return a.users[offset:end], end, end < len(a.users), nil
}

func main() {
	path := writeTempLog()
	defer os.Remove(path)

	// Lines takes an opener, not an open file. Building the pipeline
	// opens nothing; consuming it opens once; Take(2) stopping early
	// still closes.
	opens := 0
	warnings := source.Lines(func() (io.ReadCloser, error) {
		opens++
		return os.Open(path)
	}).
		Ignore().
		Filter(func(l string) bool { return strings.HasPrefix(l, "WARN") }).
		Take(2)
	fmt.Println("pipeline built, files opened:", opens)
	fmt.Println("first two warnings:", warnings.Collect())
	fmt.Println("files opened:", opens)

	// A missing file is not a panic or a special case: it is the
	// sequence's one element, an error, and the consumer decides.
	_, err := source.Lines(func() (io.ReadCloser, error) { return os.Open(path + ".missing") }).Collect()
	fmt.Println("missing file:", err != nil)

	// Paginate fetches a page only when the one before it has been used
	// up. Seven users live on four pages; the first three are on two.
	directory := &api{users: []User{
		{1, "ada"}, {2, "bob"}, {3, "cy"}, {4, "dee"}, {5, "eve"}, {6, "flo"}, {7, "gus"},
	}}
	users := source.Paginate(0, directory.list)
	first3, _ := users.Take(3).Collect()
	fmt.Printf("first three users: %v, in %d requests\n", first3, directory.requests)

	// Every iteration starts from the first cursor again, and the whole
	// directory is four requests.
	directory.requests = 0
	all, _ := users.Collect()
	fmt.Printf("all %d users, in %d requests\n", len(all), directory.requests)

	// A source is a Try, and MapTry takes one as it is: a page that
	// failed would arrive in its place, without a call made for it, and
	// the terminal still decides what a failure means.
	greet := func(_ context.Context, u User) (string, error) {
		return "hello, " + u.Name, nil
	}
	greetings, err := users.
		Then(parallel.MapTry(context.Background(), 3, greet)).
		Collect()
	fmt.Println(greetings, err)

	// CSV yields records. A row someone broke by hand is an element of
	// its own — a *csv.ParseError naming the line — and the rows after it
	// still come, so CollectAll keeps every good one.
	csvPath := writeTemp("*.csv", "sku,qty\nkbd-01,3\nmse-07,2\"\nmon-27,1\n")
	defer os.Remove(csvPath)
	stock, bad := source.CSV(func() (io.ReadCloser, error) { return os.Open(csvPath) }, nil).
		Drop(1). // the header
		CollectAll()
	fmt.Println("stock rows:", stock)
	fmt.Println("bad rows:  ", bad)

	// JSONLines decodes one value per line, the same way: a line cut
	// off mid-write is an error naming its line, not the end of the file.
	type Event struct {
		User string `json:"user"`
		Page string `json:"page"`
	}
	logPath := writeTemp("*.jsonl", `{"user":"ada","page":"/pricing"}
{"user":"bob","pa
{"user":"cy","page":"/docs"}
`)
	defer os.Remove(logPath)
	events, broken := source.JSONLines[Event](func() (io.ReadCloser, error) { return os.Open(logPath) }).
		CollectAll()
	fmt.Println("events:", events, "broken:", broken)

	// catena/source/sqlsource has the same producer for database/sql:
	//
	//	orders := sqlsource.Rows(
	//		func() (*sql.Rows, error) { return db.QueryContext(ctx, query) },
	//		func(rows *sql.Rows) (Order, error) { ... rows.Scan ... },
	//	)
	//
	// The query runs when the sequence is consumed, and the rows are
	// closed on every exit.
}

// writeTemp writes body to a new temporary file and returns its path.
func writeTemp(pattern, body string) string {
	f, err := os.CreateTemp("", "catena-example-"+pattern)
	if err != nil {
		panic(err)
	}
	f.WriteString(body)
	f.Close()
	return f.Name()
}

func writeTempLog() string {
	f, err := os.CreateTemp("", "catena-example-*.log")
	if err != nil {
		panic(err)
	}
	fmt.Fprintln(f, "INFO starting up")
	fmt.Fprintln(f, "WARN disk at 81%")
	fmt.Fprintln(f, "INFO handled 42 requests")
	fmt.Fprintln(f, "WARN disk at 92%")
	fmt.Fprintln(f, "WARN disk at 97%")
	f.Close()
	return f.Name()
}
