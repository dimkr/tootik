/*
Copyright 2023 - 2025 Dima Krasner

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

package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dimkr/tootik/ap"
)

// Unfollow queues an Undo activity for delivery.
func Unfollow(ctx context.Context, db *sql.DB, follower, followed, followID string) error {
	if followed == follower {
		return fmt.Errorf("%s cannot unfollow %s", follower, followed)
	}

	undoID, err := NewID(follower, "undo")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(followed)

	unfollow := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      undoID,
		Type:    ap.Undo,
		Actor:   follower,
		Object: &ap.Activity{
			ID:     followID,
			Type:   ap.Follow,
			Actor:  follower,
			Object: followed,
		},
		To: to,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// mark the matching Follow as received
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE outbox SET sent = 1 WHERE activity->>'$.object.id' = ? and activity->>'$.type' = 'Follow'`,
		followID,
	); err != nil {
		return fmt.Errorf("failed to mark follow activity as received: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (id, activity, sender) VALUES (?, JSONB(?), ?)`,
		unfollow.ID,
		&unfollow,
		follower,
	); err != nil {
		return fmt.Errorf("failed to insert undo for %s: %w", followID, err)
	}

	if _, err := tx.ExecContext(ctx, `delete from follows where id = ?`, followID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to unfollow %s: %w", followID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s failed to unfollow %s: %w", follower, followed, err)
	}

	return nil
}
