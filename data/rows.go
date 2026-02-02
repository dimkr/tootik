package data

import (
	"context"
	"database/sql"
	"reflect"
	"unsafe"
)

// QueryCollectRowsCountIgnore runs a SQL query.
//
// count is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func QueryCollectRowsCountIgnore[T any](
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

// QueryCollectRowsIgnore runs a SQL query.
//
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func QueryCollectRowsIgnore[T any](
	ctx context.Context,
	db *sql.DB,
	ignore func(error) bool,
	query string,
	args ...any,
) ([]T, error) {
	return QueryCollectRowsCountIgnore[T](
		ctx,
		db,
		1,
		ignore,
		query,
		args...,
	)
}

// QueryCollectRows runs a SQL query.
//
// The columns of each row are assigned to visible fields of T.
func QueryCollectRows[T any](
	ctx context.Context,
	db *sql.DB,
	query string,
	args ...any,
) ([]T, error) {
	return QueryCollectRowsCountIgnore[T](
		ctx,
		db,
		1,
		func(error) bool { return false },
		query,
		args...,
	)
}

func structFieldPtrs(t reflect.Type, base unsafe.Pointer) []any {
	fields := reflect.VisibleFields(t)
	ptrs := make([]any, len(fields))
	for i, field := range fields {
		ptrs[i] = reflect.NewAt(field.Type, unsafe.Add(base, field.Offset)).Interface()
	}

	return ptrs
}

func scanStructRows[T any](
	t reflect.Type,
	rows *sql.Rows,
	collect func(T) bool,
	ignore func(error) bool,
) error {
	var zero, row T

	ptrs := structFieldPtrs(t, unsafe.Pointer(&row))

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

	return rows.Err()
}

func scanStructPointerRows[T any](
	et reflect.Type,
	rows *sql.Rows,
	collect func(T) bool,
	ignore func(error) bool,
) error {
	size := et.Size()
	zero := make([]byte, size)
	var row = reflect.New(et)

	base := row.UnsafePointer()
	ptrs := structFieldPtrs(et, base)

	rowb := unsafe.Slice((*byte)(base), size)
	rowp := *(*T)(unsafe.Pointer(&base))

	for rows.Next() {
		copy(rowb, zero)

		if err := rows.Scan(ptrs...); err != nil {
			if !ignore(err) {
				return err
			}

			continue
		}

		if !collect(rowp) {
			break
		}
	}

	return rows.Err()
}

func scanScalarRows[T any](
	rows *sql.Rows,
	collect func(T) bool,
	ignore func(error) bool,
) error {
	var zero, row T
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

	return rows.Err()
}

// ScanRows calls a function for every result of a SQL query.
//
// collect is called for each row.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// If T is a struct or a struct pointer, the columns of each row are assigned to visible fields of T.
//
// If T is a struct pointer, collect receives the same pointer every time it's called.
// It must not store the pointer to the struct or its fields.
func ScanRows[T any](
	rows *sql.Rows,
	collect func(T) bool,
	ignore func(error) bool,
) error {
	t := reflect.TypeFor[T]()

	switch t.Kind() {
	case reflect.Struct:
		return scanStructRows(t, rows, collect, ignore)

	case reflect.Bool,
		reflect.Int,
		reflect.Int8,
		reflect.Int16,
		reflect.Int32,
		reflect.Int64,
		reflect.Uint,
		reflect.Uint8,
		reflect.Uint16,
		reflect.Uint32,
		reflect.Uint64,
		reflect.Uintptr,
		reflect.Float32,
		reflect.Float64,
		reflect.Complex64,
		reflect.Complex128,
		reflect.String:
		return scanScalarRows(rows, collect, ignore)

	case reflect.Pointer:
		et := t.Elem()
		if et.Kind() != reflect.Struct {
			panic("T must point to a struct")
		}

		return scanStructPointerRows(et, rows, collect, ignore)

	default:
		panic("T must be a struct, struct pointer or scalar")
	}
}

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

// QueryScanRows is like [ScanRows] but also runs the query.
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
