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

func (inbox *Inbox) updateNote(ctx context.Context, actor *ap.Actor, key httpsig.Key, note *ap.Object) error {
	updateID, err := inbox.NewID(note.AttributedTo, "update")
	if err != nil {
		return err
	}

	update := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:     updateID,
		Type:   ap.Update,
		Actor:  note.AttributedTo,
		Object: note,
		To:     note.To,
		CC:     note.CC,
	}

	if inbox.Config.DisableIntegrityProofs {
		note.Proof = ap.Proof{}
	} else {
		if note.Proof, err = proof.Create(key, note); err != nil {
			return err
		}

		if update.Proof, err = proof.Create(key, update); err != nil {
			return err
		}
	}

	j, err := json.Marshal(update)
	if err != nil {
		return err
	}

	tx, err := inbox.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	s := string(j)

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		s,
		note.AttributedTo,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `delete from hashtags where note = ?`, note.ID); err != nil {
		return err
	}

	for _, hashtag := range note.Tag {
		if hashtag.Type != ap.Hashtag || len(hashtag.Name) <= 1 || hashtag.Name[0] != '#' {
			continue
		}

		if _, err = tx.ExecContext(ctx, `insert into hashtags (note, hashtag) values(?, ?)`, note.ID, hashtag.Name[1:]); err != nil {
			return err
		}
	}

	if err := inbox.ProcessActivity(ctx, tx, actor, update, s, 1, false); err != nil {
		return err
	}

	return tx.Commit()
}

// UpdateNote queues an Update activity for delivery.
func (inbox *Inbox) UpdateNote(ctx context.Context, actor *ap.Actor, key httpsig.Key, note *ap.Object) error {
	if err := inbox.updateNote(ctx, actor, key, note); err != nil {
		return fmt.Errorf("failed to update %s by %s: %w", note.ID, actor.ID, err)
	}

	return nil
}

func (inbox *Inbox) updateActor(ctx context.Context, tx *sql.Tx, actor *ap.Actor, key httpsig.Key) error {
	updateID, err := inbox.NewID(actor.ID, "update")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(ap.Public)

	update := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:     updateID,
		Type:   ap.Update,
		Actor:  actor.ID,
		Object: actor.ID,
		To:     to,
	}

	if inbox.Config.DisableIntegrityProofs {
		actor.Proof = ap.Proof{}
	} else {
		if actor.Proof, err = proof.Create(key, actor); err != nil {
			return err
		}

		if update.Proof, err = proof.Create(key, update); err != nil {
			return err
		}
	}

	if _, err = tx.ExecContext(
		ctx,
		`UPDATE persons SET actor = JSONB(?) WHERE id = ?`,
		actor,
		actor.ID,
	); err != nil {
		return err
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		update,
		actor.ID,
	)
	return err
}

// UpdateActorTx queues an Update activity for delivery.
func (inbox *Inbox) UpdateActorTx(ctx context.Context, tx *sql.Tx, actor *ap.Actor, key httpsig.Key) error {
	if err := inbox.updateActor(ctx, tx, actor, key); err != nil {
		return fmt.Errorf("failed to update %s: %w", actor.ID, err)
	}

	return nil
}

// UpdateActor queues an Update activity for delivery.
func (inbox *Inbox) UpdateActor(ctx context.Context, actor *ap.Actor, key httpsig.Key) error {
	tx, err := inbox.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to update %s: %w", actor.ID, err)
	}
	defer tx.Rollback()

	if err := inbox.updateActor(ctx, tx, actor, key); err != nil {
		return fmt.Errorf("failed to update %s: %w", actor.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to update %s: %w", actor.ID, err)
	}

	return nil
}
