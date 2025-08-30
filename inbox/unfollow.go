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

	"github.com/dimkr/tootik/ap"
)

func (q *Queue) unfollow(ctx context.Context, db *sql.DB, follower *ap.Actor, followed, followID string) error {
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
		return err
	}
	defer tx.Rollback()

	// mark the matching Follow as received
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE outbox SET sent = 1 WHERE activity->>'$.object.id' = ? and activity->>'$.type' = 'Follow'`,
		followID,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		ap.Canonical(follower.ID),
	); err != nil {
		return err
	}

	if err := q.ProcessLocalActivity(ctx, tx, follower, &unfollow, string(j)); err != nil {
		return err
	}

	return tx.Commit()
}

// Unfollow queues an Undo activity for delivery.
func (q *Queue) Unfollow(ctx context.Context, db *sql.DB, follower *ap.Actor, followed, followID string) error {
	if err := q.unfollow(ctx, db, follower, followed, followID); err != nil {
		return fmt.Errorf("failed to unfollow %s from %s by %s: %w", followID, follower.ID, followed, err)
	}

	return nil
}
