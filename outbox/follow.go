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
	"errors"
	"fmt"

	"github.com/dimkr/tootik/ap"
)

// Follow queues a Follow activity for delivery.
func Follow(ctx context.Context, domain string, follower *ap.Actor, followed string, db *sql.DB) error {
	if followed == follower.ID {
		return fmt.Errorf("%s cannot follow %s", follower.ID, followed)
	}

	followID, err := NewID(follower.ID, domain, "follow")
	if err != nil {
		return err
	}

	to := ap.Audience{}
	to.Add(followed)

	follow := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      followID,
		Type:    ap.Follow,
		Actor:   follower.ID,
		Object:  followed,
		To:      to,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// if the followed user is local and doesn't require manual approval, we can mark as accepted
	if res, err := tx.ExecContext(
		ctx,
		`INSERT INTO follows
			(
				id,
				follower,
				followed,
				followedcid,
				accepted
			)
		SELECT
			$1,
			$2,
			id,
			$3,
			CASE WHEN ed25519privkey IS NOT NULL AND COALESCE(actor->>'$.manuallyApprovesFollowers', 0) = 0 THEN 1 ELSE NULL END
		FROM persons
		WHERE cid = $3
		ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = NULL, inserted = UNIXEPOCH()
		`,
		followID,
		follower.ID,
		ap.Canonical(followed),
	); err != nil {
		return fmt.Errorf("failed to insert follow: %w", err)
	} else if n, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("failed to insert follow: %w", err)
	} else if n == 0 {
		return errors.New("failed to insert follow: no rows inserted")
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (cid, activity, sender) VALUES (?, JSONB(?), ?)`,
		ap.Canonical(follow.ID),
		&follow,
		follower.ID,
	); err != nil {
		return fmt.Errorf("failed to insert follow activity: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("%s failed to follow %s: %w", follower.ID, followed, err)
	}

	return nil
}
