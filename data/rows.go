package data

import (
	"context"
	"database/sql"
	"reflect"
	"unsafe"
)

// QueryRowsCountIgnore runs a SQL query.
//
// count is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func QueryRowsCountIgnore[T any](
	ctx context.Context,
	db *sql.DB,
	count int,
	ignore func(error) bool,
	query string,
	args ...any,
) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	return ReadRows[T](rows, count, ignore)
}

// QueryRowsIgnore runs a SQL query.
//
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func QueryRowsIgnore[T any](
	ctx context.Context,
	db *sql.DB,
	ignore func(error) bool,
	query string,
	args ...any,
) ([]T, error) {
	return QueryRowsCountIgnore[T](
		ctx,
		db,
		10,
		ignore,
		query,
		args...,
	)
}

// QueryRows runs a SQL query.
//
// The columns of each row are assigned to visible fields of T.
func QueryRows[T any](
	ctx context.Context,
	db *sql.DB,
	query string,
	args ...any,
) ([]T, error) {
	return QueryRowsCountIgnore[T](
		ctx,
		db,
		10,
		func(error) bool { return false },
		query,
		args...,
	)
}

// ReadRows reads the results of a SQL query.
//
// expected is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func ReadRows[T any](rows *sql.Rows, expected int, ignore func(error) bool) ([]T, error) {
	dest := make([]T, expected)
	count := 0

	if t := reflect.TypeFor[T](); t.Kind() == reflect.Struct {
		fields := reflect.VisibleFields(t)

		ptrs := make([]any, len(fields))

		for rows.Next() {
			if count == cap(dest) {
				dest = append(dest, dest[0])[:count]
			}

			base := unsafe.Pointer(&dest[count])
			for i, field := range fields {
				ptrs[i] = reflect.NewAt(field.Type, unsafe.Add(base, field.Offset)).Interface()
			}

			if err := rows.Scan(ptrs...); err != nil {
				if !ignore(err) {
					return nil, err
				}

				continue
			}

			count++
		}
	} else {
		for rows.Next() {
			if count == cap(dest) {
				dest = append(dest, dest[0])[:count]
			}

			if err := rows.Scan(&dest[count]); err != nil {
				if !ignore(err) {
					return nil, err
				}

				continue
			}

			count++
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return dest[:count], nil
}
