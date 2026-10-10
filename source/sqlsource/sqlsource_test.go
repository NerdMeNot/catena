package sqlsource_test

// Rows is tested through the real database/sql stack, against a minimal
// in-memory driver: the lifecycle that matters — when the query runs, and
// that the driver's rows are closed exactly once on every exit — is
// database/sql's, so a fake of *sql.Rows would prove nothing.

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/NerdMeNot/catena"
	"github.com/NerdMeNot/catena/source/sqlsource"
)

var errBoom = errors.New("boom")

// The driver serves the integers 0..n-1 for the query "n"; "n!k" fails
// the result set at row k, and "fail" fails to run at all.
var queries, closes int

type memDriver struct{}
type memConn struct{}
type memStmt struct{ query string }
type memRows struct{ i, n, failAt int }

func (memDriver) Open(string) (driver.Conn, error)         { return memConn{}, nil }
func (memConn) Prepare(q string) (driver.Stmt, error)      { return memStmt{q}, nil }
func (memConn) Close() error                               { return nil }
func (memConn) Begin() (driver.Tx, error)                  { return nil, driver.ErrSkip }
func (memStmt) Close() error                               { return nil }
func (memStmt) NumInput() int                              { return 0 }
func (memStmt) Exec([]driver.Value) (driver.Result, error) { return nil, driver.ErrSkip }
func (s memStmt) Query([]driver.Value) (driver.Rows, error) {
	queries++
	if s.query == "fail" {
		return nil, errBoom
	}
	r := &memRows{failAt: -1}
	if _, err := fmt.Sscanf(strings.ReplaceAll(s.query, "!", " "), "%d %d", &r.n, &r.failAt); err != nil {
		fmt.Sscanf(s.query, "%d", &r.n)
	}
	return r, nil
}
func (*memRows) Columns() []string { return []string{"v"} }
func (*memRows) Close() error      { closes++; return nil }
func (r *memRows) Next(dest []driver.Value) error {
	if r.i == r.failAt {
		return errBoom
	}
	if r.i >= r.n {
		return io.EOF
	}
	dest[0] = int64(r.i)
	r.i++
	return nil
}

func init() { sql.Register("sqlsource-mem", memDriver{}) }

func ints(db *sql.DB, query string) catena.Try[int] {
	return sqlsource.Rows(
		func() (*sql.Rows, error) { return db.Query(query) },
		func(rows *sql.Rows) (int, error) {
			var v int
			if err := rows.Scan(&v); err != nil {
				return 0, err
			}
			if v == 2 {
				return 99, errors.New("row 2 does not scan") // a value alongside an error must not get through
			}
			return v, nil
		},
	)
}

func TestRows(t *testing.T) {
	db, err := sql.Open("sqlsource-mem", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1) // a leaked result set would deadlock the next query

	lifecycle := func(t *testing.T, wantQueries, wantCloses int, f func()) {
		t.Helper()
		q, c := queries, closes
		f()
		if queries-q != wantQueries || closes-c != wantCloses {
			t.Fatalf("queries=%d closes=%d, want %d and %d", queries-q, closes-c, wantQueries, wantCloses)
		}
	}

	t.Run("never consumed, never queried", func(t *testing.T) {
		lifecycle(t, 0, 0, func() { _ = ints(db, "5").Ignore().Take(1) })
	})

	t.Run("full drain: a failed scan is an element, and iteration continues", func(t *testing.T) {
		lifecycle(t, 1, 1, func() {
			vals, errs := ints(db, "5").CollectAll()
			if !slices.Equal(vals, []int{0, 1, 3, 4}) || len(errs) != 1 {
				t.Fatalf("vals=%v errs=%v", vals, errs)
			}
		})
	})

	t.Run("a failed scan yields the zero value", func(t *testing.T) {
		for v, err := range ints(db, "5") {
			if err != nil && v != 0 {
				t.Fatalf("value %d alongside an error", v)
			}
		}
	})

	t.Run("early termination closes", func(t *testing.T) {
		lifecycle(t, 1, 1, func() {
			if got := ints(db, "100").Ignore().Take(2).Collect(); !slices.Equal(got, []int{0, 1}) {
				t.Fatalf("got %v", got)
			}
		})
	})

	t.Run("open failure is the only element", func(t *testing.T) {
		lifecycle(t, 1, 0, func() {
			vals, errs := ints(db, "fail").CollectAll()
			if vals != nil || len(errs) != 1 || !errors.Is(errs[0], errBoom) {
				t.Fatalf("vals=%v errs=%v", vals, errs)
			}
		})
	})

	t.Run("an error ending the result set is yielded last", func(t *testing.T) {
		lifecycle(t, 1, 1, func() {
			vals, err := ints(db, "10!1").Collect()
			if !slices.Equal(vals, []int{0}) || !errors.Is(err, errBoom) {
				t.Fatalf("vals=%v err=%v", vals, err)
			}
		})
	})

	t.Run("each iteration queries afresh", func(t *testing.T) {
		lifecycle(t, 2, 2, func() {
			rows := ints(db, "2")
			first, _ := rows.Collect()
			second, _ := rows.Collect()
			if !slices.Equal(first, second) {
				t.Fatalf("%v then %v", first, second)
			}
		})
	})
}

func ExampleRows() {
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
	// Output: [{0} {1}] <nil>
}
