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
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func (inbox *Inbox) announce(ctx context.Context, tx *sql.Tx, actor *ap.Actor, key httpsig.Key, note *ap.Object) error {
	announceID, err := inbox.NewID(actor.ID, "announce")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	to.Add(note.AttributedTo)
	to.Add(actor.Followers)

	announce := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:        announceID,
		Type:      ap.Announce,
		Actor:     actor.ID,
		Published: ap.Time{Time: time.Now()},
		To:        to,
		CC:        cc,
		Object:    note.ID,
	}

	if !inbox.Config.DisableIntegrityProofs {
		if announce.Proof, err = proof.Create(key, announce); err != nil {
			return err
		}
	}

	s, err := danger.MarshalJSON(announce)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender, inserted) VALUES (JSONB(?), ?, ?)`,
		s,
		actor.ID,
		time.Now().UnixNano(),
	); err != nil {
		return err
	}

	return inbox.ProcessActivity(
		ctx,
		tx,
		sql.NullString{},
		actor,
		announce,
		s,
		1,
		false,
	)
}

// Announce queues an Announce activity for delivery.
func (inbox *Inbox) Announce(ctx context.Context, tx *sql.Tx, actor *ap.Actor, key httpsig.Key, note *ap.Object) error {
	if err := inbox.announce(ctx, tx, actor, key, note); err != nil {
		return fmt.Errorf("failed to announce %s by %s: %w", note.ID, actor.ID, err)
	}

	return nil
}
