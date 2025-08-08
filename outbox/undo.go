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

package outbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

// Undo queues an Undo activity for delivery.
func Undo(ctx context.Context, domain string, db *sql.DB, activity *ap.Activity, key httpsig.Key) error {
	noteID, ok := activity.Object.(string)
	if !ok {
		return errors.New("cannot undo activity")
	}

	id, err := NewID(domain, "undo")
	if err != nil {
		return err
	}

	to := activity.To
	to.Add(ap.Public)

	undo := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      id,
		Type:    ap.Undo,
		Actor:   activity.Actor,
		To:      to,
		CC:      activity.CC,
		Object:  activity,
	}

	if key.ID != "" {
		undo.Context = []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v1"}

		var err error
		undo.Proof, err = proof.Create(key, time.Now(), &undo, undo.Context)
		if err != nil {
			return err
		}
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
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
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
