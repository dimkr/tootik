/*
Copyright 2023, 2024 Dima Krasner

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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
)

// CreateNobody creates the special "nobdoy" user.
// This user is used to sign outgoing requests not initiated by a particular user.
func CreateNobody(ctx context.Context, domain string, db *sql.DB) (*ap.Actor, error) {
	var actor ap.Actor
	if err := db.QueryRowContext(ctx, `select actor from persons where actor->>'preferredUsername' = 'nobody' and host = ?`, domain).Scan(&actor); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to create nobody user: %w", err)
	} else if err == nil {
		return &actor, nil
	}

	return Create(ctx, domain, db, "nobody", "")
}
