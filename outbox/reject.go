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
func Reject(ctx context.Context, domain string, followed, follower string, db *sql.DB) error {
	id, err := NewID(domain, "reject")
	if err != nil {
		return err
	}

	recipients := ap.Audience{}
	recipients.Add(follower)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	}
	defer tx.Rollback()

	var followID string
	if err := tx.QueryRowContext(
		ctx,
		`SELECT id FROM follows WHERE follower = ? and followed = ? AND accepted IS NULL`,
		follower,
		followed,
	).Scan(&followID); err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	}

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
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		&reject,
		followed,
	); err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE follows SET accepted = 0 WHERE id = ?`,
		followID,
	); err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to reject %s: %w", follower, err)
	}

	return nil
}
