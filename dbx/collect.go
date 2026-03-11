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

import "database/sql"

// CollectRows reads the results of a SQL query.
//
// expected is the expected number of rows.
// ignore determines which [sql.Rows.Scan] errors should be ignored.
//
// If T is a struct, the columns of each row are assigned to visible fields of T.
//
// T must not be a pointer.
func CollectRows[T any](rows *sql.Rows, expected int, ignore func(error) bool) ([]T, error) {
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
