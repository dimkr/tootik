package dbx

import "database/sql"

// ReadRows reads the results of a SQL query.
//
// expected is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// If T is a struct, the columns of each row are assigned to visible fields of T.
//
// T must not be a pointer.
func ReadRows[T any](rows *sql.Rows, expected int, ignore func(error) bool) ([]T, error) {
	scanned := make([]T, 0, expected)

	if err := ScanRows(
		rows,
		func(row T) {
			scanned = append(scanned, row)
		},
		ignore,
	); err != nil {
		return nil, err
	}

	return scanned, nil
}
