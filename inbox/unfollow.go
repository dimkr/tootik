/*
Copyright 2023 - 2026 Dima Krasner

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

func (inbox *Inbox) unfollow(ctx context.Context, follower *ap.Actor, key httpsig.Key, followed, followID string) error {
	if ap.Canonical(followed) == ap.Canonical(follower.ID) {
		return fmt.Errorf("%s cannot unfollow %s", follower.ID, followed)
	}

	undoID, err := inbox.NewID(follower.ID, "undo")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(followed)

	unfollow := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:    undoID,
		Type:  ap.Undo,
		Actor: follower.ID,
		Object: &ap.Activity{
			ID:     followID,
			Type:   ap.Follow,
			Actor:  follower.ID,
			Object: followed,
		},
		To: to,
	}

	if !inbox.Config.DisableIntegrityProofs {
		if unfollow.Proof, err = proof.Create(key, unfollow); err != nil {
			return err
		}
	}

	s, err := danger.MarshalJSON(unfollow)
	if err != nil {
		return err
	}

	tx, err := inbox.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// mark the matching Follow as received
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE outbox SET sent = 1 WHERE activity->>'$.object.id' = ? and activity->>'$.type' = 'Follow'`,
		followID,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender, inserted) VALUES (JSONB(?), ?, ?)`,
		s,
		follower.ID,
		time.Now().UnixNano(),
	); err != nil {
		return err
	}

	if err := inbox.ProcessActivity(
		ctx,
		tx,
		sql.NullString{},
		follower,
		unfollow,
		s,
		1,
		false,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// Unfollow queues an Undo activity for delivery.
func (inbox *Inbox) Unfollow(ctx context.Context, follower *ap.Actor, key httpsig.Key, followed, followID string) error {
	if err := inbox.unfollow(ctx, follower, key, followed, followID); err != nil {
		return fmt.Errorf("failed to unfollow %s from %s by %s: %w", followID, follower.ID, followed, err)
	}

	return nil
}
