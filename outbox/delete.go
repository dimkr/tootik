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

package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
)

func Delete(ctx context.Context, db *sql.DB, note *ap.Object) error {
	delete, err := json.Marshal(ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      note.ID + "#delete",
		Type:    ap.DeleteActivity,
		Actor:   note.AttributedTo,
		Object: ap.Object{
			Type: note.Type,
			ID:   note.ID,
		},
		To: note.To,
		CC: note.CC,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal delete: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// mark this post as sent so recipients who haven't received it yet don't receive it
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE outbox SET sent = 1 WHERE activity->>'object.id' = ? and activity->>'type' = 'Create'`,
		note.ID,
	); err != nil {
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
		`DELETE FROM outbox WHERE activity->>'object.id' = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to delete activities: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (?,?)`,
		string(delete),
		note.AttributedTo,
	); err != nil {
		return fmt.Errorf("failed to insert delete activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to delete note: %w", err)
	}

	return nil
}
