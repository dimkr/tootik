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

func (inbox *Inbox) accept(
	ctx context.Context,
	actor *ap.Actor,
	key httpsig.Key,
	request *ap.Activity,
	result string,
	tx *sql.Tx,
) (*ap.Activity, string, error) {
	id, err := inbox.NewID(actor.ID, "accept")
	if err != nil {
		return nil, "", err
	}

	recipients := ap.Audience{}
	recipients.Add(request.Actor)

	accept := &ap.Activity{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		Type:   ap.Accept,
		ID:     id,
		Actor:  actor.ID,
		To:     recipients,
		Object: request,
		Result: result,
	}

	if !inbox.Config.DisableIntegrityProofs {
		if accept.Proof, err = proof.Create(key, accept); err != nil {
			return nil, "", err
		}
	}

	s, err := danger.MarshalJSON(accept)
	if err != nil {
		return nil, "", err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender, inserted) VALUES (JSONB(?), ?, ?)`,
		s,
		actor.ID,
		time.Now().UnixNano(),
	); err != nil {
		return nil, "", err
	}

	return accept, s, nil
}

// Accept queues an Accept activity for delivery.
func (inbox *Inbox) AcceptFollow(
	ctx context.Context,
	followed *ap.Actor,
	key httpsig.Key,
	follower, followID string,
	tx *sql.Tx,
) error {
	if accept, raw, err := inbox.accept(
		ctx,
		followed,
		key,
		&ap.Activity{
			Actor:  follower,
			Type:   ap.Follow,
			Object: followed,
			ID:     followID,
		},
		"",
		tx,
	); err != nil {
		return fmt.Errorf("failed to accept %s from %s by %s: %w", followID, follower, followed.ID, err)
	} else if err := inbox.ProcessActivity(
		ctx,
		tx,
		sql.NullString{},
		followed,
		accept,
		raw,
		1,
		false,
	); err != nil {
		return fmt.Errorf("failed to accept %s from %s by %s: %w", followID, follower, followed.ID, err)
	}

	return nil
}

func (inbox *Inbox) acceptRequest(
	ctx context.Context,
	actor *ap.Actor,
	key httpsig.Key,
	request *ap.Activity,
	tx *sql.Tx,
) error {
	var instrumentID string
	switch v := request.Instrument.(type) {
	case *ap.Object:
		instrumentID = v.ID
	case string:
		instrumentID = v
	default:
		return fmt.Errorf("invalid instrument type: %T", request.Instrument)
	}

	stampID, err := inbox.NewID(actor.ID, "stamp")
	if err != nil {
		return err
	}

	if _, _, err := inbox.accept(
		ctx,
		actor,
		key,
		&ap.Activity{
			Type:       request.Type,
			ID:         request.ID,
			Actor:      request.Actor,
			Object:     request.Object,
			Instrument: instrumentID,
		},
		stampID,
		tx,
	); err != nil {
		return fmt.Errorf("failed to accept %s from %s by %s: %w", request.ID, request.Actor, actor.ID, err)
	}

	return nil
}
