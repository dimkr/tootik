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
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"time"
)

func Edit(ctx context.Context, db *sql.DB, note *ap.Object, newContent string) error {
	now := time.Now()

	note.Content = newContent
	note.Updated = &now

	body, err := json.Marshal(note)
	if err != nil {
		return fmt.Errorf("Failed to marshal note: %w", err)
	}

	updateID := fmt.Sprintf("https://%s/update/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%d", note.ID, now.Unix()))))

	update, err := json.Marshal(ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      updateID,
		Type:    ap.UpdateActivity,
		Actor:   note.AttributedTo,
		Object:  note,
		To:      note.To,
		CC:      note.CC,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal update: %w", err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE notes SET object = ? WHERE id = ?`,
		string(body),
		note.ID,
	); err != nil {
		return fmt.Errorf("Failed to update note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity) VALUES(?)`,
		string(update),
	); err != nil {
		return fmt.Errorf("Failed to insert update activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("Failed to update note: %w", err)
	}

	return nil
}
