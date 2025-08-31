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

func (inbox *Inbox) announce(ctx context.Context, tx *sql.Tx, actor *ap.Actor, note *ap.Object) error {
	announceID, err := inbox.NewID(actor.ID, "announce")
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
		Published: ap.Time{Time: time.Now()},
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
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		actor.ID,
	); err != nil {
		return err
	}

	return inbox.ProcessActivity(ctx, tx, actor, &announce, string(j), 1, false)
}

// Announce queues an Announce activity for delivery.
func (inbox *Inbox) Announce(ctx context.Context, tx *sql.Tx, actor *ap.Actor, note *ap.Object) error {
	if err := inbox.announce(ctx, tx, actor, note); err != nil {
		return fmt.Errorf("failed to announce %s by %s: %w", note.ID, actor.ID, err)
	}

	return nil
}
