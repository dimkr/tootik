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
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func (inbox *Inbox) accept(ctx context.Context, followed *ap.Actor, key httpsig.Key, follower, followID string, tx *sql.Tx) error {
	id, err := inbox.NewID(followed.ID, "accept")
	if err != nil {
		return err
	}

	recipients := ap.Audience{}
	recipients.Add(follower)

	accept := &ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		Type:    ap.Accept,
		ID:      id,
		Actor:   followed.ID,
		To:      recipients,
		Object: &ap.Activity{
			Actor:  follower,
			Type:   ap.Follow,
			Object: followed,
			ID:     followID,
		},
	}

	if !inbox.Config.DisableIntegrityProofs {
		if accept.Proof, err = proof.Create(key, accept); err != nil {
			return err
		}
	}

	j, err := json.Marshal(accept)
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

	return inbox.ProcessActivity(ctx, tx, followed, accept, string(j), 1, false)
}

// Accept queues an Accept activity for delivery.
func (inbox *Inbox) Accept(ctx context.Context, followed *ap.Actor, key httpsig.Key, follower, followID string, tx *sql.Tx) error {
	if err := inbox.accept(ctx, followed, key, follower, followID, tx); err != nil {
		return fmt.Errorf("failed to accept %s from %s by %s: %w", followID, follower, followed.ID, err)
	}

	return nil
}
