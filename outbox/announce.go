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
	"fmt"
	"github.com/dimkr/tootik/ap"
	"time"
)

// Announce queues an Announce activity for delivery.
func Announce(ctx context.Context, domain string, tx *sql.Tx, actor *ap.Actor, note *ap.Object) error {
	now := time.Now()
	announceID := fmt.Sprintf("https://%s/announce/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", actor.ID, note.ID, now.UnixNano()))))

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
