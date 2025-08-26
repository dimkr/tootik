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
)

// Accept queues an Accept activity for delivery.
func (q *Queue) Accept(ctx context.Context, followed *ap.Actor, follower, followID string, tx *sql.Tx) error {
	id, err := q.NewID(followed.ID, "accept")
	if err != nil {
		return err
	}

	recipients := ap.Audience{}
	recipients.Add(follower)

	accept := ap.Activity{
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

	j, err := json.Marshal(&accept)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (cid, activity, sender) VALUES (?, JSONB(?), ?)`,
		ap.Canonical(accept.ID),
		string(j),
		followed.ID,
	); err != nil {
		return fmt.Errorf("failed to accept %s: %w", followID, err)
	}

	if err := q.ProcessLocalActivity(ctx, tx, followed, &accept, string(j)); err != nil {
		return fmt.Errorf("failed to accept %s: %w", followID, err)
	}

	return nil
}
