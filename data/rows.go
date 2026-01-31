package data

import (
	"context"
	"database/sql"
	"reflect"
	"unsafe"
)

// CollectRowsCountIgnore runs a SQL query.
//
// count is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func CollectRowsCountIgnore[T any](
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

// CollectRowsIgnore runs a SQL query.
//
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func CollectRowsIgnore[T any](
	ctx context.Context,
	db *sql.DB,
	ignore func(error) bool,
	query string,
	args ...any,
) ([]T, error) {
	return CollectRowsCountIgnore[T](
		ctx,
		db,
		10,
		ignore,
		query,
		args...,
	)
}

// CollectRows runs a SQL query.
//
// The columns of each row are assigned to visible fields of T.
func CollectRows[T any](
	ctx context.Context,
	db *sql.DB,
	query string,
	args ...any,
) ([]T, error) {
	return CollectRowsCountIgnore[T](
		ctx,
		db,
		10,
		func(error) bool { return false },
		query,
		args...,
	)
}

// ScanRows calls a function for every result of a SQL query.
//
// collect is called for each row.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// If T is a struct, the columns of each row are assigned to visible fields of T.
func ScanRows[T any](
	rows *sql.Rows,
	collect func(T) bool,
	ignore func(error) bool,
) error {
	var zero, row T

	if t := reflect.TypeFor[T](); t.Kind() == reflect.Struct {
		fields := reflect.VisibleFields(t)
		ptrs := make([]any, len(fields))
		base := unsafe.Pointer(&row)
		for i, field := range fields {
			ptrs[i] = reflect.NewAt(field.Type, unsafe.Add(base, field.Offset)).Interface()
		}

		for rows.Next() {
			row = zero

			if err := rows.Scan(ptrs...); err != nil {
				if !ignore(err) {
					return err
				}

				continue
			}

			if !collect(row) {
				break
			}
		}
	} else {
		var rowp any = &row

		for rows.Next() {
			row = zero

			if err := rows.Scan(rowp); err != nil {
				if !ignore(err) {
					return err
				}

				continue
			}

			if !collect(row) {
				break
			}
		}
	}

	return rows.Err()
}

// ReadRows reads the results of a SQL query.
//
// expected is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// If T is a struct, the columns of each row are assigned to visible fields of T.
func ReadRows[T any](rows *sql.Rows, expected int, ignore func(error) bool) ([]T, error) {
	scanned := make([]T, 0, expected)

	if err := ScanRows(
		rows,
		func(row T) bool {
			scanned = append(scanned, row)
			return true
		},
		ignore,
	); err != nil {
		return nil, err
	}

	return scanned, nil
}

func QueryScanRows[T any](
	ctx context.Context,
	collect func(T) bool,
	ignore func(error) bool,
	db *sql.DB,
	query string,
	args ...any,
) error {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()

	return ScanRows(rows, collect, ignore)
}
