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

package fed

import (
	"context"
	"database/sql"
	"errors"

	"golang.org/x/crypto/acme/autocert"
)

type dbAutoCertCache struct {
	*sql.DB
}

func (c dbAutoCertCache) Get(ctx context.Context, key string) ([]byte, error) {
	var data []byte
	if err := c.QueryRowContext(
		ctx,
		`select data from autocert_cache where key = ?`,
		key,
	).Scan(&data); errors.Is(err, sql.ErrNoRows) {
		return nil, autocert.ErrCacheMiss
	} else if err != nil {
		return nil, err
	} else {
		return data, nil
	}
}

func (c dbAutoCertCache) Put(ctx context.Context, key string, data []byte) error {
	_, err := c.ExecContext(ctx, `insert into autocert_cache(key, data) values(?, ?)`, key, data)
	return err
}

func (c dbAutoCertCache) Delete(ctx context.Context, key string) error {
	_, err := c.ExecContext(ctx, `delete from autocert_cache where key = ?`, key)
	return err
}
