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
	"encoding/json"
	"errors"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/inbox/note"
)

const maxDeliveryQueueSize = 128

var ErrDeliveryQueueFull = errors.New("delivery queue is full")

// Create queues a Create activity for delivery.
func Create(ctx context.Context, domain string, cfg *cfg.Config, db *sql.DB, post *ap.Object, author *ap.Actor) error {
	id, err := NewID(domain, "create")
	if err != nil {
		return err
	}

	var queueSize int
	if err := db.QueryRowContext(ctx, `select count(distinct activity->>'$.id') from outbox where sent = 0 and attempts < ?`, cfg.MaxDeliveryAttempts).Scan(&queueSize); err != nil {
		return fmt.Errorf("failed to query delivery queue size: %w", err)
	}

	if queueSize >= maxDeliveryQueueSize {
		return ErrDeliveryQueueFull
	}

	create := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		Type:    ap.Create,
		ID:      id,
		Actor:   author.ID,
		Object:  post,
		To:      post.To,
		CC:      post.CC,
	}

	j, err := json.Marshal(create)
	if err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := note.Insert(ctx, tx, post); err != nil {
		return fmt.Errorf("failed to insert note: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `insert into feed(follower, note, author, inserted) values(?, jsonb(?), jsonb(?), unixepoch())`, author.ID, post, author); err != nil {
		return fmt.Errorf("failed to insert Create: %w", err)
	}

	if _, err = tx.ExecContext(ctx, `insert into outbox (activity, sender) values (jsonb(?),?)`, string(j), author.ID); err != nil {
		return fmt.Errorf("failed to insert Create: %w", err)
	}

	if err := ForwardActivity(ctx, domain, cfg, tx, post, &create, string(j)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to create note: %w", err)
	}

	return nil
}
