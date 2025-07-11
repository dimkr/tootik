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
	"encoding/json"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
)

// Delete queues a Delete activity for delivery.
func Delete(ctx context.Context, domain string, cfg *cfg.Config, db *sql.DB, note *ap.Object) error {
	delete := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      note.ID + "#delete",
		Type:    ap.Delete,
		Actor:   note.AttributedTo,
		Object: ap.Object{
			Type: note.Type,
			ID:   note.ID,
		},
		To: note.To,
		CC: note.CC,
	}

	j, err := json.Marshal(delete)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// mark this post as sent so recipients who haven't received it yet don't receive it
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE outbox SET sent = 1 WHERE activity->>'$.object.id' = ? AND activity->>'$.type' = 'Create'`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to insert delete activity: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		note.AttributedTo,
	); err != nil {
		return fmt.Errorf("failed to insert delete activity: %w", err)
	}

	// determine whether or not this activity needs to be forwarded before we potentially punch a hole in the thread
	if err := ForwardActivity(ctx, domain, cfg, tx, note, &delete, string(j)); err != nil {
		return fmt.Errorf("failed to insert delete activity: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM notes WHERE id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM notesfts WHERE id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM shares WHERE note = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM bookmarks WHERE note = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM feed WHERE note->>'$.id' = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	return nil
}
