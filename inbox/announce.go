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

package inbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
)

// Announce queues an Announce activity for delivery.
func (q *Queue) Announce(ctx context.Context, tx *sql.Tx, actor *ap.Actor, note *ap.Object) error {
	now := time.Now()
	announceID, err := q.NewID(actor.ID, "announce")
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
		Published: ap.Time{Time: now},
		To:        to,
		CC:        cc,
		Object:    note.ID,
	}

	j, err := json.Marshal(&announce)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (cid, activity, sender) VALUES (?, JSONB(?), ?)`,
		ap.Canonical(announce.ID),
		string(j),
		actor.ID,
	); err != nil {
		return fmt.Errorf("failed to insert announce activity: %w", err)
	}

	if err := q.ProcessLocalActivity(ctx, tx, actor, &announce, string(j)); err != nil {
		return fmt.Errorf("failed to insert announce activity: %w", err)
	}

	return nil
}
