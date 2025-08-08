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

package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

// Accept queues an Accept activity for delivery.
func Accept(ctx context.Context, domain string, followed, follower, followID string, tx *sql.Tx, key httpsig.Key) error {
	id, err := NewID(domain, "accept")
	if err != nil {
		return err
	}

	recipients := ap.Audience{}
	recipients.Add(follower)

	accept := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		Type:    ap.Accept,
		ID:      id,
		Actor:   followed,
		To:      recipients,
		Object: &ap.Activity{
			Actor:  follower,
			Type:   ap.Follow,
			Object: followed,
			ID:     followID,
		},
	}

	if key.ID != "" {
		accept.Context = []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v1"}

		accept.Proof, err = proof.Create(key, time.Now(), &accept, accept.Context)
		if err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		&accept,
		followed,
	); err != nil {
		return fmt.Errorf("failed to accept %s: %w", followID, err)
	}

	if res, err := tx.ExecContext(
		ctx,
		`INSERT INTO follows (id, follower, followed, accepted) VALUES($1, $2, $3, 1) ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = 1, inserted = UNIXEPOCH()`,
		followID,
		follower,
		followed,
	); err != nil {
		return fmt.Errorf("failed to accept %s: %w", followID, err)
	} else if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("failed to accept %s: %w", followID, err)
	} else if n == 0 {
		return fmt.Errorf("failed to accept %s: cannot accept", followID)
	}

	return nil
}
