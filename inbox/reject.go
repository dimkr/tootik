/*
Copyright 2025 Dima Krasner

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
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func (inbox *Inbox) reject(ctx context.Context, followed *ap.Actor, key httpsig.Key, follower, followID string, tx *sql.Tx) error {
	id, err := inbox.NewID(followed.ID, "reject")
	if err != nil {
		return err
	}

	recipients := ap.Audience{}
	recipients.Add(follower)

	reject := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		Type:  ap.Reject,
		ID:    id,
		Actor: followed.ID,
		To:    recipients,
		Object: &ap.Activity{
			Actor:  follower,
			Type:   ap.Follow,
			Object: followed,
			ID:     followID,
		},
	}

	if !inbox.Config.DisableIntegrityProofs {
		if reject.Proof, err = proof.Create(key, reject); err != nil {
			return err
		}
	}

	j, err := json.Marshal(reject)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		followed.ID,
	); err != nil {
		return err
	}

	return inbox.ProcessActivity(ctx, tx, followed, reject, string(j), 1, false)
}

// Reject queues a Reject activity for delivery.
func (inbox *Inbox) Reject(ctx context.Context, followed *ap.Actor, key httpsig.Key, follower, followID string, tx *sql.Tx) error {
	if err := inbox.reject(ctx, followed, key, follower, followID, tx); err != nil {
		return fmt.Errorf("failed to reject %s from %s by %s: %w", followID, follower, followed.ID, err)
	}

	return nil
}
