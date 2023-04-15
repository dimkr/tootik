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

package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

const (
	notesTTL    = time.Hour * 24 * 30
	deliveryTTL = time.Hour * 24
)

func CollectGarbage(ctx context.Context, db *sql.DB) error {
	now := time.Now()

	if _, err := db.ExecContext(ctx, `delete from notes where inserted < ?`, now.Add(-notesTTL).Unix()); err != nil {
		return fmt.Errorf("Failed to remove old posts: %w", err)
	}

	if _, err := db.ExecContext(ctx, `delete from hashtags where note in (select distinct hashtags.note from hashtags left join notes on notes.id = hashtags.note where notes.id is null)`); err != nil {
		return fmt.Errorf("Failed to remove old hashtags: %w", err)
	}

	if _, err := db.ExecContext(ctx, `delete from deliveries where inserted < ?`, now.Add(-deliveryTTL).Unix()); err != nil {
		return fmt.Errorf("Failed to remove old posts: %w", err)
	}

	return nil
}
