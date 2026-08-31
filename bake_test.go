package catena_test

// The bake-period pipelines (spec §13, post-v0.1): the library against
// real machinery rather than fixtures.
//
// Pipeline 1 drives the §4.2 lazy-acquisition Rows pattern through the
// REAL database/sql stack — a minimal in-memory driver.Driver serves the
// rows, so sql.Rows lifecycle, Scan conversion errors, iteration errors,
// and Close-on-early-termination are all the standard library's own code
// paths, not mocks of them.
//
// Pipeline 2 crunches 200k synthetic log lines through bufio scanning,
// Try parsing with positional error wrapping, FoldBy aggregation, TopNBy
// selection, and a JoinBy against a user table — verified against truth
// computed while generating the data.

import (
	"bufio"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"io"
	"math/rand"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/NerdMeNot/catena"
)

// ---- minimal database/sql driver ----

var memRowsClosed atomic.Int32

type memDriver struct{}

type memConn struct{}

type memStmt struct{ rows [][]driver.Value }

type memRows struct {
	cols   []string
	rows   [][]driver.Value
	i      int
	errAt  int // Next returns an error at this row index (-1: never)
	rowErr error
}

var memData [][]driver.Value

func (memDriver) Open(string) (driver.Conn, error) { return memConn{}, nil }

func (memConn) Prepare(string) (driver.Stmt, error) { return memStmt{memData}, nil }
func (memConn) Close() error                        { return nil }
func (memConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (memStmt) Close() error  { return nil }
func (memStmt) NumInput() int { return 0 }
func (memStmt) Exec([]driver.Value) (driver.Result, error) {
	return nil, driver.ErrSkip
}
func (s memStmt) Query([]driver.Value) (driver.Rows, error) {
	return &memRows{cols: []string{"id", "amount"}, rows: s.rows, errAt: -1}, nil
}

func (r *memRows) Columns() []string { return r.cols }
func (r *memRows) Close() error {
	memRowsClosed.Add(1)
	return nil
}
func (r *memRows) Next(dest []driver.Value) error {
	if r.i == r.errAt {
		return r.rowErr
	}
	if r.i >= len(r.rows) {
		return io.EOF
	}
	copy(dest, r.rows[r.i])
	r.i++
	return nil
}

func init() {
	sql.Register("catena-mem", memDriver{})
}

type row struct {
	ID     int
	Amount int
}

func sqlRows(db *sql.DB, query string) catena.Try[row] {
	return Rows(
		func() (*sql.Rows, error) { return db.Query(query) },
		func(rs *sql.Rows) (row, error) {
			var r row
			err := rs.Scan(&r.ID, &r.Amount)
			return r, err
		},
	)
}

func TestBakeSQLPipeline(t *testing.T) {
	db, err := sql.Open("catena-mem", "")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	memData = nil
	for i := range 1000 {
		memData = append(memData, []driver.Value{int64(i), int64((i * 37) % 500)})
	}

	t.Run("full_drain_aggregation", func(t *testing.T) {
		before := memRowsClosed.Load()
		totals := sqlRows(db, "SELECT id, amount FROM orders").
			Must().
			FoldBy(
				func(r row) int { return r.ID % 7 },
				func(int) int { return 0 },
				func(sum int, r row) int { return sum + r.Amount },
			)
		wantTotals := map[int]int{}
		for i := range 1000 {
			wantTotals[i%7] += (i * 37) % 500
		}
		for k, want := range wantTotals {
			if totals[k] != want {
				t.Fatalf("bucket %d: got %d, want %d", k, totals[k], want)
			}
		}
		if memRowsClosed.Load() != before+1 {
			t.Fatalf("driver rows not closed exactly once (%d)", memRowsClosed.Load()-before)
		}
	})

	t.Run("early_termination_closes_rows", func(t *testing.T) {
		before := memRowsClosed.Load()
		got := sqlRows(db, "SELECT id, amount FROM orders").Ignore().Take(3).Collect()
		if len(got) != 3 {
			t.Fatalf("got %v", got)
		}
		if memRowsClosed.Load() != before+1 {
			t.Fatal("early termination through Ignore().Take(3) did not close the driver rows — §4.2 broken against real database/sql")
		}
	})

	t.Run("scan_error_policy", func(t *testing.T) {
		// a value Scan cannot convert to int
		memData[500] = []driver.Value{"not-an-int", int64(0)}
		defer func() { memData[500] = []driver.Value{int64(500), int64(0)} }()

		vals, err := sqlRows(db, "SELECT id, amount FROM orders").
			WrapErr(func(err error) error { return fmt.Errorf("orders scan: %w", err) }).
			Collect()
		if err == nil || !strings.Contains(err.Error(), "orders scan:") {
			t.Fatalf("wrapped scan error not surfaced: %v", err)
		}
		if len(vals) != 500 {
			t.Fatalf("partial collect has %d rows, want the 500 before the bad one", len(vals))
		}

		// skip-and-continue over the same corruption
		good := sqlRows(db, "SELECT id, amount FROM orders").Ignore().Count()
		if good != 999 {
			t.Fatalf("Ignore kept %d rows, want 999", good)
		}
	})

	t.Run("never_consumed_never_opens", func(t *testing.T) {
		before := memRowsClosed.Load()
		_ = sqlRows(db, "SELECT id, amount FROM orders").Ignore().Take(3) // built, not consumed
		if memRowsClosed.Load() != before {
			t.Fatal("building an unconsumed pipeline touched the database")
		}
	})
}

// ---- log-crunch pipeline ----

type logEntry struct {
	Method string
	Path   string
	Status int
	User   int
	Ms     int
}

func parseLogLine(line string) (logEntry, error) {
	var e logEntry
	_, err := fmt.Sscanf(line, "%s %s %d user=%d ms=%d",
		&e.Method, &e.Path, &e.Status, &e.User, &e.Ms)
	return e, err
}

func TestBakeLogPipeline(t *testing.T) {
	const lines = 200_000
	rng := rand.New(rand.NewSource(42))
	paths := []string{"/api/orders", "/api/users", "/health", "/api/search", "/login"}
	users := map[int]string{0: "ada", 1: "bob", 2: "eve", 3: "kim"}

	// generate the log AND the ground truth in the same pass
	var sb strings.Builder
	truthStatus := map[int]int{}
	truthPathMs := map[string]int{}
	truthUserHits := map[string]int{}
	malformed := 0
	for i := range lines {
		if i%10_000 == 9_999 { // sprinkle corruption
			sb.WriteString("!!corrupted line!!\n")
			malformed++
			continue
		}
		method := "GET"
		if rng.Intn(4) == 0 {
			method = "POST"
		}
		path := paths[rng.Intn(len(paths))]
		status := []int{200, 200, 200, 404, 500}[rng.Intn(5)]
		user := rng.Intn(len(users))
		ms := rng.Intn(900) + 1
		fmt.Fprintf(&sb, "%s %s %d user=%d ms=%d\n", method, path, status, user, ms)
		truthStatus[status]++
		if status == 200 {
			truthPathMs[path] += ms
		}
		truthUserHits[users[user]]++
	}
	logText := sb.String()

	entries := func() catena.Try[logEntry] {
		return func(yield func(logEntry, error) bool) {
			sc := bufio.NewScanner(strings.NewReader(logText))
			line := 0
			for sc.Scan() {
				line++
				e, err := parseLogLine(sc.Text())
				if err != nil {
					var zero logEntry
					e = zero
					err = fmt.Errorf("line %d: %w", line, err)
				}
				if !yield(e, err) {
					return
				}
			}
		}
	}

	start := time.Now()

	// error accounting: every corrupted line surfaces, positioned
	_, errs := entries().CollectAll()
	if len(errs) != malformed {
		t.Fatalf("saw %d errors, want %d", len(errs), malformed)
	}
	if !strings.Contains(errs[0].Error(), "line 10000:") {
		t.Fatalf("first error lacks position: %v", errs[0])
	}

	ok := entries().Ignore()

	// status tally via streaming FoldBy
	statusCounts := ok.FoldBy(
		func(e logEntry) int { return e.Status },
		func(int) int { return 0 },
		func(n int, _ logEntry) int { return n + 1 },
	)
	for k, want := range truthStatus {
		if statusCounts[k] != want {
			t.Fatalf("status %d: got %d, want %d", k, statusCounts[k], want)
		}
	}

	// slowest endpoints by total latency on 200s only
	pathMs := ok.
		Filter(func(e logEntry) bool { return e.Status == 200 }).
		FoldBy(
			func(e logEntry) string { return e.Path },
			func(string) int { return 0 },
			func(sum int, e logEntry) int { return sum + e.Ms },
		)
	type pathTotal struct {
		Path string
		Ms   int
	}
	top3 := catena.FromMap(pathMs).
		MapTo(func(p string, ms int) pathTotal { return pathTotal{p, ms} }).
		TopNBy(3, func(pt pathTotal) int { return pt.Ms })
	wantTop := catena.FromMap(truthPathMs).
		MapTo(func(p string, ms int) pathTotal { return pathTotal{p, ms} }).
		TopNBy(3, func(pt pathTotal) int { return pt.Ms })
	if !slices.Equal(top3, wantTop) {
		t.Fatalf("top endpoints: got %v, want %v", top3, wantTop)
	}

	// JoinBy against the user table: hits per user name
	type userRow struct {
		ID   int
		Name string
	}
	userTable := catena.FromMap(users).MapTo(func(id int, name string) userRow {
		return userRow{id, name}
	})
	hitsByName := ok.
		JoinBy(userTable,
			func(e logEntry) int { return e.User },
			func(u userRow) int { return u.ID },
			func(_ logEntry, u userRow) string { return u.Name },
		).
		FoldBy(catena.Self[string],
			func(string) int { return 0 },
			func(n int, _ string) int { return n + 1 },
		)
	for name, want := range truthUserHits {
		if hitsByName[name] != want {
			t.Fatalf("user %s: got %d, want %d", name, hitsByName[name], want)
		}
	}

	elapsed := time.Since(start)
	t.Logf("log pipeline: %d lines, 4 passes (incl. join) in %v (%.0f klines/s per pass)",
		lines, elapsed, 4*float64(lines)/elapsed.Seconds()/1000)
}
