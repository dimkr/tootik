/*
Copyright 2024 Dima Krasner

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
	"time"
)

// Undo queues an Undo activity for delivery.
func Undo(ctx context.Context, domain string, db *sql.DB, activity *ap.Activity) error {
	noteID, ok := activity.Object.(string)
	if !ok {
		return errors.New("cannot undo activity")
	}

	activity.To.Add(ap.Public)

	undo := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      fmt.Sprintf("https://%s/undo/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%d", activity.ID, time.Now().UnixNano())))),
		Type:    ap.Undo,
		Actor:   activity.Actor,
		To:      activity.To,
		CC:      activity.CC,
		Object:  activity,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM shares WHERE note = ? AND by = ?`,
		noteID,
		activity.Actor,
	); err != nil {
		return fmt.Errorf("failed to remove share: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`DELETE FROM feed WHERE note->>'$.id' = ? AND sharer->>'$.id' = ?`,
		noteID,
		activity.Actor,
	); err != nil {
		return fmt.Errorf("failed to remove share: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		&undo,
		activity.Actor,
	); err != nil {
		return fmt.Errorf("failed to insert undo activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s failed to undo %s: %w", activity.Actor, activity.ID, err)
	}

	return nil
}
