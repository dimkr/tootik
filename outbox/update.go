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
	"fmt"
	"github.com/dimkr/tootik/ap"
	inote "github.com/dimkr/tootik/inbox/note"
	"time"
)

func UpdateNote(ctx context.Context, domain string, db *sql.DB, note *ap.Object) error {
	updateID := fmt.Sprintf("https://%s/update/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%d", note.ID, time.Now().UnixNano()))))

	update := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      updateID,
		Type:    ap.UpdateActivity,
		Actor:   note.AttributedTo,
		Object:  note,
		To:      note.To,
		CC:      note.CC,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE notes SET object = ? WHERE id = ?`,
		note,
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
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		&update,
		note.AttributedTo,
	); err != nil {
		return fmt.Errorf("failed to insert update activity: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`update notes set to0 = object->>'to[0]' where id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update to0: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`update notes set to1 = object->>'to[1]' where id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update to1: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`update notes set to2 = object->>'to[2]' where id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update to2: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`update notes set cc0 = object->>'cc[0]' where id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update cc0: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`update notes set cc1 = object->>'cc[1]' where id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update cc1: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`update notes set cc2 = object->>'cc[2]' where id = ?`,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update cc2: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `delete from hashtags where note = ?`, note.ID); err != nil {
		return fmt.Errorf("failed to delete old hashtags: %w", err)
	}

	for _, hashtag := range note.Tag {
		if hashtag.Type != ap.HashtagMention || len(hashtag.Name) <= 1 || hashtag.Name[0] != '#' {
			continue
		}
		if _, err = tx.ExecContext(ctx, `insert into hashtags (note, hashtag) values(?,?)`, note.ID, hashtag.Name[1:]); err != nil {
			return fmt.Errorf("failed to insert hashtag: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	return nil
}

func UpdateActor(ctx context.Context, domain string, tx *sql.Tx, actorID string) error {
	updateID := fmt.Sprintf("https://%s/update/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%d", actorID, time.Now().UnixNano()))))

	to := ap.Audience{}
	to.Add(ap.Public)

	update := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      updateID,
		Type:    ap.UpdateActivity,
		Actor:   actorID,
		Object:  actorID,
		To:      to,
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		&update,
		actorID,
	); err != nil {
		return fmt.Errorf("failed to insert update activity: %w", err)
	}

	return nil
}
