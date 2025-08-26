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

package inbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/dimkr/tootik/ap"
)

// Unfollow queues an Undo activity for delivery.
func (q *Queue) Unfollow(ctx context.Context, db *sql.DB, follower *ap.Actor, followed, followID string) error {
	if ap.Canonical(followed) == ap.Canonical(follower.ID) {
		return fmt.Errorf("%s cannot unfollow %s", follower.ID, followed)
	}

	undoID, err := q.NewID(follower.ID, "undo")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(followed)

	unfollow := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      undoID,
		Type:    ap.Undo,
		Actor:   follower.ID,
		Object: &ap.Activity{
			ID:     followID,
			Type:   ap.Follow,
			Actor:  follower.ID,
			Object: followed,
		},
		To: to,
	}

	j, err := json.Marshal(&unfollow)
	if err != nil {
		return err
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
		`INSERT INTO outbox (cid, activity, sender) VALUES (?, JSONB(?), ?)`,
		ap.Canonical(unfollow.ID),
		string(j),
		follower.ID,
	); err != nil {
		return fmt.Errorf("failed to insert undo for %s: %w", followID, err)
	}

	if err := q.processActivity(ctx, tx, slog.With(), follower, &unfollow, string(j), 1, false); err != nil {
		return fmt.Errorf("failed to insert undo for %s: %w", followID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to insert undo for %s: %w", followID, err)
	}

	return nil
}
