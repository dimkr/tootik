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

package data

import (
	"context"
	"database/sql"
	"errors"
	"reflect"

	"golang.org/x/crypto/acme/autocert"
)

var _ autocert.Cache = Cache[struct{}]{}

// Cache stores module-scoped cache in a database.
//
// It satisfies [autocert.Cache].
type Cache[T any] struct {
	DB interface {
		QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
		ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	}
}

func (c Cache[T]) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	if err := c.DB.QueryRowContext(
		ctx,
		`select data from cache where module = ? || '.' || ? and key = ?`,
		reflect.TypeFor[T]().PkgPath(),
		reflect.TypeFor[T]().Name(),
		key,
	).Scan(&data); errors.Is(err, sql.ErrNoRows) {
		return nil, autocert.ErrCacheMiss
	} else if err != nil {
		return nil, err
	} else {
		return data, nil
	}
}

func (c Cache[T]) Put(ctx context.Context, key string, data []byte) error {
	_, err := c.DB.ExecContext(
		ctx,
		`insert into cache(module, key, data) values(? || '.' || ?, ?, ?)`,
		reflect.TypeFor[T]().PkgPath(),
		reflect.TypeFor[T]().Name(),
		key,
		data,
	)
	return err
}

func (c Cache[T]) Delete(ctx context.Context, key string) error {
	_, err := c.DB.ExecContext(
		ctx,
		`delete from cache where module = ? || '.' || ? and key = ?`,
		reflect.TypeFor[T]().PkgPath(),
		reflect.TypeFor[T]().Name(),
		key,
	)
	return err
}
