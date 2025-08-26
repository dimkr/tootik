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

func (q *Queue) follow(ctx context.Context, follower *ap.Actor, followed string, db *sql.DB) error {
	if followed == follower.ID {
		return fmt.Errorf("%s cannot follow %s", follower.ID, followed)
	}

	followID, err := q.NewID(follower.ID, "follow")
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

	j, err := json.Marshal(&follow)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (cid, activity, sender) VALUES (?, JSONB(?), ?)`,
		ap.Canonical(follow.ID),
		string(j),
		follower.ID,
	); err != nil {
		return err
	}

	if err := q.ProcessLocalActivity(ctx, tx, follower, &follow, string(j)); err != nil {
		return err
	}

	return tx.Commit()
}

// Follow queues a Follow activity for delivery.
func (q *Queue) Follow(ctx context.Context, follower *ap.Actor, followed string, db *sql.DB) error {
	if err := q.follow(ctx, follower, followed, db); err != nil {
		return fmt.Errorf("failed to follow %s by %s: %w", followed, follower.ID, err)
	}

	return nil
}
