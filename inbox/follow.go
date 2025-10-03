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
	"encoding/json"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func (inbox *Inbox) follow(ctx context.Context, follower *ap.Actor, key httpsig.Key, followed string) error {
	if followed == follower.ID {
		return fmt.Errorf("%s cannot follow %s", follower.ID, followed)
	}

	followID, err := inbox.NewID(follower.ID, "follow")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(followed)

	follow := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:     followID,
		Type:   ap.Follow,
		Actor:  follower.ID,
		Object: followed,
		To:     to,
	}

	if !inbox.Config.DisableIntegrityProofs {
		if follow.Proof, err = proof.Create(key, follow); err != nil {
			return err
		}
	}

	j, err := json.Marshal(follow)
	if err != nil {
		return err
	}

	tx, err := inbox.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		follower.ID,
	); err != nil {
		return err
	}

	if err := inbox.ProcessActivity(ctx, tx, follower, follow, string(j), 1, false); err != nil {
		return err
	}

	return tx.Commit()
}

// Follow queues a Follow activity for delivery.
func (inbox *Inbox) Follow(ctx context.Context, follower *ap.Actor, key httpsig.Key, followed string) error {
	if err := inbox.follow(ctx, follower, key, followed); err != nil {
		return fmt.Errorf("failed to follow %s by %s: %w", followed, follower.ID, err)
	}

	return nil
}
