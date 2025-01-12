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
	"crypto/sha256"
	"database/sql"
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
)

// Accept queues an Accept activity for delivery.
func Accept(ctx context.Context, domain string, followed, follower, followID string, db *sql.DB) error {
	recipients := ap.Audience{}
	recipients.Add(follower)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	accept := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		Type:    ap.Accept,
		ID:      fmt.Sprintf("https://%s/accept/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", followed, follower, time.Now().UnixNano())))),
		Actor:   followed,
		To:      recipients,
		Object: &ap.Activity{
			Type: ap.Follow,
			ID:   followID,
		},
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		&accept,
		followed,
	); err != nil {
		return fmt.Errorf("failed to insert Accept: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO follows (id, follower, followed, accepted) VALUES(?,?,?,?)`,
		followID,
		follower,
		followed,
		1,
	); err != nil {
		return fmt.Errorf("failed to insert follow %s: %w", followID, err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to accept follow: %w", err)
	}

	return nil
}
