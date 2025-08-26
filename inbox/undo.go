/*
Copyright 2024, 2025 Dima Krasner

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

// Undo queues an Undo activity for delivery.
func (q *Queue) Undo(ctx context.Context, db *sql.DB, actor *ap.Actor, activity *ap.Activity) error {
	id, err := q.NewID(actor.ID, "undo")
	if err != nil {
		return err
	}

	to := activity.To
	to.Add(ap.Public)

	undo := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      id,
		Type:    ap.Undo,
		Actor:   actor.ID,
		To:      to,
		CC:      activity.CC,
		Object:  activity,
	}

	j, err := json.Marshal(&undo)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (cid, activity, sender) VALUES (?, JSONB(?), ?)`,
		ap.Canonical(undo.ID),
		string(j),
		activity.Actor,
	); err != nil {
		return fmt.Errorf("failed to insert undo activity: %w", err)
	}

	if err := q.ProcessLocalActivity(ctx, tx, actor, &undo, string(j)); err != nil {
		return fmt.Errorf("failed to insert undo activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s failed to undo %s: %w", activity.Actor, activity.ID, err)
	}

	return nil
}
