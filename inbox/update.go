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

func (q *Queue) updateNote(ctx context.Context, db *sql.DB, actor *ap.Actor, note *ap.Object) error {
	updateID, err := q.NewID(note.AttributedTo, "update")
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
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		note.AttributedTo,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `delete from hashtags where note = ?`, note.ID); err != nil {
		return err
	}

	for _, hashtag := range note.Tag {
		if hashtag.Type != ap.Hashtag || len(hashtag.Name) <= 1 || hashtag.Name[0] != '#' {
			continue
		}

		if _, err = tx.ExecContext(ctx, `insert into hashtags (note, hashtag) values(?, ?)`, note.ID, hashtag.Name[1:]); err != nil {
			return err
		}
	}

	if err := q.ProcessLocalActivity(ctx, tx, actor, &update, string(j)); err != nil {
		return err

	}

	return tx.Commit()
}

// UpdateNote queues an Update activity for delivery.
func (q *Queue) UpdateNote(ctx context.Context, db *sql.DB, actor *ap.Actor, note *ap.Object) error {
	if err := q.updateNote(ctx, db, actor, note); err != nil {
		return fmt.Errorf("failed to update %s by %s: %w", note.ID, actor.ID, err)
	}

	return nil
}

func (q *Queue) updateActor(ctx context.Context, tx *sql.Tx, actorID string) error {
	updateID, err := q.NewID(actorID, "update")
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

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		&update,
		actorID,
	)
	return err
}

// UpdateActor queues an Update activity for delivery.
func (q *Queue) UpdateActor(ctx context.Context, tx *sql.Tx, actorID string) error {
	if err := q.updateActor(ctx, tx, actorID); err != nil {
		return fmt.Errorf("failed to update %s: %w", actorID, err)
	}

	return nil
}
