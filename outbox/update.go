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
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/text/plain"
	"strings"
	"time"
)

func UpdateNote(ctx context.Context, db *sql.DB, note *ap.Object) error {
	body, err := json.Marshal(note)
	if err != nil {
		return fmt.Errorf("failed to marshal note: %w", err)
	}

	updateID := fmt.Sprintf("https://%s/update/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%d", note.ID, time.Now().UnixNano()))))

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
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	content, links := plain.FromHTML(note.Content)
	if len(links) > 0 {
		var b strings.Builder
		appended := false
		b.WriteString(content)
		links.Range(func(link, alt string) bool {
			if alt == "" {
				return true
			}
			if !appended {
				b.WriteString(content)
			}
			b.WriteByte(' ')
			b.WriteString(link)
			appended = true
			return true
		})
		if appended {
			content = b.String()
		}
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE notes SET object = ? WHERE id = ?`,
		string(body),
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE notesfts SET content = ? WHERE id = ?`,
		content,
		note.ID,
	); err != nil {
		return fmt.Errorf("failed to update note: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		string(update),
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

	if _, err = tx.ExecContext(
		ctx,
		`update notes
		set groupid = coalesce(
			case
				when notes.object->'inReplyTo' is null then
					(
						select value->>'href' from json_each(notes.object->'tag')
						where value->>'type' = 'Mention' and exists (select 1 from persons where persons.id = value->>'href' and persons.actor->>'type' = 'Group')
						limit 1
					)
				else
					null
			end,
			case
				when notes.object->'inReplyTo' is null then
					null
				else
					(
						select value from json_each(notes.object->'cc')
						where exists (select 1 from persons where persons.id = value and persons.actor->>'type' = 'Group')
						limit 1
					)
			end,
			case
				when notes.object->'inReplyTo' is null then
					null
				else
					(
						select value from json_each(notes.object->'to')
						where exists (select 1 from persons where persons.id = value and persons.actor->>'type' = 'Group')
						limit 1
					)
			end
		)
		where id = ?`, note.ID); err != nil {
		return fmt.Errorf("failed to update post group: %w", err)
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

func UpdateActor(ctx context.Context, tx *sql.Tx, actorID string) error {
	updateID := fmt.Sprintf("https://%s/update/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%d", actorID, time.Now().UnixNano()))))

	to := ap.Audience{}
	to.Add(ap.Public)

	update, err := json.Marshal(ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      updateID,
		Type:    ap.UpdateActivity,
		Actor:   actorID,
		Object:  actorID,
		To:      to,
	})
	if err != nil {
		return fmt.Errorf("failed to marshal update: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		string(update),
		actorID,
	); err != nil {
		return fmt.Errorf("failed to insert update activity: %w", err)
	}

	return nil
}
