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

package outbox

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/dimkr/tootik/ap"
)

// Reject queues a Reject activity for delivery.
func Reject(ctx context.Context, domain string, followed, follower, followID string, tx *sql.Tx) error {
	id, err := NewID(domain, followed, "reject")
	if err != nil {
		return err
	}

	recipients := ap.Audience{}
	recipients.Add(follower)

	reject := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		Type:    ap.Reject,
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

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES (JSONB(?), ?)`,
		&reject,
		followed,
	); err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	}

	if res, err := tx.ExecContext(
		ctx,
		`UPDATE follows SET accepted = 0 WHERE id = ? AND (accepted IS NULL OR accepted = 1)`,
		followID,
	); err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	} else if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	} else if n == 0 {
		return fmt.Errorf("failed to reject %s: cannot reject", follower)
	}

	return nil
}
