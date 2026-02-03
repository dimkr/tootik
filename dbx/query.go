package dbx

import (
	"context"
	"database/sql"
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

// QueryScanRows is like [ScanRows] but also runs the query.
func QueryScanRows[T any](
	ctx context.Context,
	collect func(T),
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
