/*
Copyright 2026 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package dbx

import (
	"context"
	"database/sql"
)

// QueryCollectCountIgnore runs a SQL query.
//
// count is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func QueryCollectCountIgnore[T any](
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

// QueryCollectIgnore runs a SQL query.
//
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// The columns of each row are assigned to visible fields of T.
func QueryCollectIgnore[T any](
	ctx context.Context,
	db *sql.DB,
	ignore func(error) bool,
	query string,
	args ...any,
) ([]T, error) {
	return QueryCollectCountIgnore[T](
		ctx,
		db,
		1,
		ignore,
		query,
		args...,
	)
}

// QueryCollect runs a SQL query.
//
// The columns of each row are assigned to visible fields of T.
func QueryCollect[T any](
	ctx context.Context,
	db *sql.DB,
	query string,
	args ...any,
) ([]T, error) {
	return QueryCollectCountIgnore[T](
		ctx,
		db,
		1,
		func(error) bool { return false },
		query,
		args...,
	)
}

// QueryScan is like [ScanRows] but also runs the query.
func QueryScan[T any](
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
