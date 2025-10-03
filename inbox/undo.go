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
	"encoding/json"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func (inbox *Inbox) undo(ctx context.Context, actor *ap.Actor, key httpsig.Key, activity *ap.Activity) error {
	id, err := inbox.NewID(actor.ID, "undo")
	if err != nil {
		return err
	}

	to := activity.To
	to.Add(ap.Public)

	undo := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:     id,
		Type:   ap.Undo,
		Actor:  actor.ID,
		To:     to,
		CC:     activity.CC,
		Object: activity,
	}

	if !inbox.Config.DisableIntegrityProofs {
		if undo.Proof, err = proof.Create(key, undo); err != nil {
			return err
		}
	}

	j, err := json.Marshal(undo)
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
		activity.Actor,
	); err != nil {
		return err
	}

	if err := inbox.ProcessActivity(ctx, tx, actor, undo, string(j), 1, false); err != nil {
		return err
	}

	return tx.Commit()
}

// Undo queues an Undo activity for delivery.
func (inbox *Inbox) Undo(ctx context.Context, actor *ap.Actor, key httpsig.Key, activity *ap.Activity) error {
	if err := inbox.undo(ctx, actor, key, activity); err != nil {
		return fmt.Errorf("failed to undo %s by %s: %w", activity.ID, actor.ID, err)
	}

	return nil
}
