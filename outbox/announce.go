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
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
)

// Announce queues an Announce activity for delivery.
func Announce(ctx context.Context, domain string, tx *sql.Tx, actor *ap.Actor, note *ap.Object) error {
	now := time.Now()
	announceID, err := NewID(domain, "announce")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	to.Add(note.AttributedTo)
	to.Add(actor.Followers)

	announce := ap.Activity{
		Context:   "https://www.w3.org/ns/activitystreams",
		ID:        announceID,
		Type:      ap.Announce,
		Actor:     actor.ID,
		Published: &ap.Time{Time: now},
		To:        to,
		CC:        cc,
		Object:    note.ID,
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO shares (note, by) VALUES(?,?)`,
		note.ID,
		actor.ID,
	); err != nil {
		return fmt.Errorf("failed to insert share: %w", err)
	}

	if actor.Type == ap.Person {
		if _, err := tx.ExecContext(
			ctx,
			`
			INSERT INTO feed (follower, note, author, sharer, inserted)
			SELECT $1, $2, authors.actor, $3, UNIXEPOCH()
			FROM persons authors
			WHERE authors.id = $4
			`,
			actor.ID,
			note,
			actor,
			note.AttributedTo,
		); err != nil {
			return fmt.Errorf("failed to insert announce activity: %w", err)
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		&announce,
		actor.ID,
	); err != nil {
		return fmt.Errorf("failed to insert announce activity: %w", err)
	}

	return nil
}
