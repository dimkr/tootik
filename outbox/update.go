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
	inote "github.com/dimkr/tootik/inbox/note"
)

// UpdateNote queues an Update activity for delivery.
func UpdateNote(ctx context.Context, domain string, cfg *cfg.Config, db *sql.DB, note *ap.Object) error {
	updateID, err := NewID(domain, note.AttributedTo, "update")
	if err != nil {
		return err
	}

	update := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      updateID,
		Type:    ap.Update,
		Actor:   note.AttributedTo,
		Object:  note,
		To:      note.To,
		CC:      note.CC,
	}

	j, err := json.Marshal(update)
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
		`UPDATE notes SET object = JSONB(?) WHERE id = ?`,
		&note,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE notesfts SET content = ? WHERE id = ?`,
		inote.Flatten(note),
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE feed SET note = JSONB(?) WHERE note->>'$.id' = ?`,
		&note,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		note.AttributedTo,
	); err != nil {
		return fmt.Errorf("failed to insert update activity: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `delete from hashtags where note = ?`, note.ID); err != nil {
		return fmt.Errorf("failed to delete old hashtags: %w", err)
	}

	for _, hashtag := range note.Tag {
		if hashtag.Type != ap.Hashtag || len(hashtag.Name) <= 1 || hashtag.Name[0] != '#' {
			continue
		}
		if _, err = tx.ExecContext(ctx, `insert into hashtags (note, hashtag) values(?,?)`, note.ID, hashtag.Name[1:]); err != nil {
			return fmt.Errorf("failed to insert hashtag: %w", err)
		}
	}

	if err := ForwardActivity(ctx, domain, cfg, tx, note, &update, string(j)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	return nil
}

// UpdateActor queues an Update activity for delivery.
func UpdateActor(ctx context.Context, domain string, tx *sql.Tx, actorID string) error {
	updateID, err := NewID(domain, actorID, "update")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(ap.Public)

	update := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      updateID,
		Type:    ap.Update,
		Actor:   actorID,
		Object:  actorID,
		To:      to,
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		&update,
		actorID,
	); err != nil {
		return fmt.Errorf("failed to insert update activity: %w", err)
	}

	return nil
}
