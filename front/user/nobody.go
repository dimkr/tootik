/*
Copyright 2023 Dima Krasner

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

package user

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
)

func CreateNobody(ctx context.Context, db *sql.DB) (*ap.Actor, error) {
	id := fmt.Sprintf("https://%s/user/nobody", cfg.Domain)

	var actorString string
	if err := db.QueryRowContext(ctx, `select actor from persons where id = ?`, id).Scan(&actorString); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to create nobody user: %w", err)
	} else if err == nil {
		var actor ap.Actor
		if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
			return nil, err
		}
		return &actor, nil
	}

	return Create(ctx, db, id, "nobody", "")
}
