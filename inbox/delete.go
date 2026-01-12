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

func (inbox *Inbox) delete(ctx context.Context, actor *ap.Actor, key httpsig.Key, note *ap.Object) error {
	delete := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:    note.ID + "#delete",
		Type:  ap.Delete,
		Actor: note.AttributedTo,
		Object: &ap.Object{
			Type: note.Type,
			ID:   note.ID,
		},
		To: note.To,
		CC: note.CC,
	}

	if !inbox.Config.DisableIntegrityProofs {
		var err error
		if delete.Proof, err = proof.Create(key, delete); err != nil {
			return err
		}
	}

	s, err := danger.MarshalJSON(delete)
	if err != nil {
		return err
	}

	tx, err := inbox.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// mark this post as sent so recipients who haven't received it yet don't receive it
	if _, err := tx.ExecContext(
		ctx,
		`UPDATE outbox SET sent = 1 WHERE activity->>'$.object.id' = ? AND activity->>'$.type' = 'Create'`,
		note.ID,
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender, inserted) VALUES (JSONB(?), ?, ?)`,
		s,
		note.AttributedTo,
		time.Now().UnixNano(),
	); err != nil {
		return err
	}

	if err := inbox.ProcessActivity(
		ctx,
		tx,
		sql.NullString{},
		actor,
		delete,
		s,
		1,
		false,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// Delete queues a Delete activity for delivery.
func (inbox *Inbox) Delete(ctx context.Context, actor *ap.Actor, key httpsig.Key, note *ap.Object) error {
	if err := inbox.delete(ctx, actor, key, note); err != nil {
		return fmt.Errorf("failed to delete %s by %s: %w", note.ID, actor.ID, err)
	}

	return nil
}
