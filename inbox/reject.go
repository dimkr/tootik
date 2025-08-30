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
)

func (q *Queue) reject(ctx context.Context, followed *ap.Actor, follower, followID string, tx *sql.Tx) error {
	id, err := q.NewID(followed.ID, "reject")
	if err != nil {
		return err
	}

	recipients := ap.Audience{}
	recipients.Add(follower)

	reject := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		Type:    ap.Reject,
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

	j, err := json.Marshal(&reject)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		string(j),
		ap.Canonical(followed.ID),
	); err != nil {
		return err
	}

	return q.ProcessLocalActivity(ctx, tx, followed, &reject, string(j))
}

// Reject queues a Reject activity for delivery.
func (q *Queue) Reject(ctx context.Context, followed *ap.Actor, follower, followID string, tx *sql.Tx) error {
	if err := q.reject(ctx, followed, follower, followID, tx); err != nil {
		return fmt.Errorf("failed to reject %s from %s by %s: %w", followID, follower, followed.ID, err)
	}

	return nil
}
