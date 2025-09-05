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
	"errors"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
)

const maxDeliveryQueueSize = 128

var ErrDeliveryQueueFull = errors.New("delivery queue is full")

func (inbox *Inbox) create(ctx context.Context, cfg *cfg.Config, db *sql.DB, post *ap.Object, author *ap.Actor) error {
	id, err := inbox.NewID(author.ID, "create")
	if err != nil {
		return err
	}

	var queueSize int
	if err := db.QueryRowContext(ctx, `select count(distinct cid) from outbox where sent = 0 and attempts < ?`, cfg.MaxDeliveryAttempts).Scan(&queueSize); err != nil {
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
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, `insert into outbox (activity, sender) values (jsonb(?),?)`, string(j), author.ID); err != nil {
		return err
	}

	if err := inbox.ProcessActivity(ctx, tx, author, &create, string(j), 1, false); err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `insert into feed(follower, note, author, inserted) values(?, jsonb(?), jsonb(?), unixepoch())`, author.ID, post, author); err != nil {
		return err
	}

	return tx.Commit()
}

// Create queues a Create activity for delivery.
func (inbox *Inbox) Create(ctx context.Context, cfg *cfg.Config, db *sql.DB, post *ap.Object, author *ap.Actor) error {
	if err := inbox.create(ctx, cfg, db, post, author); err != nil {
		return fmt.Errorf("failed to create %s by %s: %w", post.ID, author.ID, err)
	}

	return nil
}
