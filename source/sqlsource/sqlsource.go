// Package sqlsource is the database/sql producer of catena/source, in a
// package of its own so that the others do not link database/sql.
package sqlsource

import (
	"database/sql"

	"github.com/NerdMeNot/catena"
)

// Rows yields one element per row of the result set that open returns,
// each produced by scan. open is called once per iteration, inside it, so
// a pipeline that is built but never consumed runs no query; the rows are
// closed on every exit, including a downstream stage stopping early. A
// failure to open is yielded as (zero, err) and ends the sequence. A row
// that fails to scan is yielded as (zero, err) and iteration continues —
// the consumer picks the policy. An error that ends the result set early
// (rows.Err) is yielded last. scan must not call rows.Next or rows.Close.
// ⚠ Holds the connection behind the rows until the iteration returns.
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
