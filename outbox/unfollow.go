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

package outbox

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"log/slog"
	"time"
)

// Unfollow queues an Unfollow activity for delivery.
func Unfollow(ctx context.Context, domain string, log *slog.Logger, db *sql.DB, follower *ap.Actor, followed, followID string) error {
	if followed == follower.ID {
		return fmt.Errorf("%s cannot unfollow %s", follower.ID, followed)
	}

	undoID := fmt.Sprintf("https://%s/undo/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", follower.ID, followed, time.Now().UnixNano()))))

	to := ap.Audience{}
	to.Add(followed)

	unfollow := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      undoID,
		Type:    ap.UndoActivity,
		Actor:   follower.ID,
		Object: &ap.Activity{
			ID:     followID,
			Type:   ap.FollowActivity,
			Actor:  follower.ID,
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
		`UPDATE outbox SET sent = 1 WHERE activity->>'object.id' = ? and activity->>'type' = 'Follow'`,
		followID,
	); err != nil {
		return fmt.Errorf("failed to mark follow activity as received: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		&unfollow,
		follower.ID,
	); err != nil {
		return fmt.Errorf("failed to insert undo for %s: %w", followID, err)
	}

	if _, err := tx.ExecContext(ctx, `delete from follows where id = ?`, followID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to unfollow %s: %w", followID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s failed to unfollow %s: %w", follower.ID, followed, err)
	}

	return nil
}
