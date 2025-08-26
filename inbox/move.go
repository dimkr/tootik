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
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
)

// Move queues a Move activity for delivery.
func (q *Queue) Move(ctx context.Context, db *sql.DB, from *ap.Actor, to string) error {
	now := time.Now()

	aud := ap.Audience{}
	aud.Add(from.Followers)

	id, err := q.NewID(from.ID, "move")
	if err != nil {
		return err
	}

	move := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      id,
		Actor:   from.ID,
		Type:    ap.Move,
		Object:  from.ID,
		Target:  to,
		To:      aud,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`update persons set actor = jsonb_set(actor, '$.movedTo', $1, '$.updated', $2) where id = $3`,
		to,
		now.Format(time.RFC3339Nano),
		from.ID,
	); err != nil {
		return fmt.Errorf("failed to insert Move: %w", err)
	}

	if err := q.UpdateActor(ctx, tx, from.ID); err != nil {
		return fmt.Errorf("failed to insert Move: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`insert into outbox (cid, activity, sender) values (?, jsonb(?), ?)`,
		ap.Canonical(move.ID),
		&move,
		from.ID,
	); err != nil {
		return fmt.Errorf("failed to insert Move: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to insert Move: %w", err)
	}

	return nil
}
