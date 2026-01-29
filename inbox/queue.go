/*
Copyright 2023 - 2026 Dima Krasner

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
	"log/slog"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
)

type Queue struct {
	Config   *cfg.Config
	DB       *sql.DB
	Inbox    ap.Inbox
	Resolver ap.Resolver
	Keys     [2]httpsig.Key
}

type batchItem struct {
	ID             int64
	Path           sql.NullString
	Sender         sql.Null[ap.Actor]
	Activity       ap.Activity
	ActivityString string
	Shared         bool
}

func (q *Queue) processActivityWithTimeout(parent context.Context, item batchItem) {
	if !item.Sender.Valid {
		slog.Warn("Sender is unknown", "id", item.ID)
		return
	}

	ctx, cancel := context.WithTimeout(parent, q.Config.ActivityProcessingTimeout)
	defer cancel()

	if _, err := q.Resolver.ResolveID(ctx, q.Keys, item.Activity.Actor, 0); err != nil {
		slog.Warn("Failed to resolve actor", "activity", item.Activity, "error", err)
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.Warn("Failed to start transaction", "activity", item.Activity, "error", err)
		return
	}
	defer tx.Rollback()

	if err := q.Inbox.ProcessActivity(ctx, tx, item.Path, &item.Sender.V, &item.Activity, item.ActivityString, 1, item.Shared); err != nil {
		slog.Warn("Failed to process activity", "activity", item.Activity, "error", err)
	} else if err := tx.Commit(); err != nil {
		slog.Warn("Failed to commit changes", "activity", item.Activity, "error", err)
	}
}

// ProcessBatch processes one batch of incoming activites in the queue.
func (q *Queue) ProcessBatch(ctx context.Context) (int, error) {
	slog.Debug("Polling activities queue")

	batch, err := data.QueryRowsCountIgnore[batchItem](
		ctx,
		q.DB,
		q.Config.ActivitiesBatchSize,
		func(err error) bool {
			slog.Error("Failed to scan activity", "error", err)
			return true
		},
		`select inbox.id, inbox.path, json(persons.actor), json(inbox.activity), inbox.raw, inbox.raw->>'$.type' = 'Announce' as shared from (select * from inbox limit -1 offset case when (select count(*) from inbox) >= $1 then $1/10 else 0 end) inbox left join persons on persons.id = inbox.sender order by inbox.id limit $2`,
		q.Config.MaxActivitiesQueueSize,
		q.Config.ActivitiesBatchSize,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch activities to process: %w", err)
	}

	if len(batch) == 0 {
		return 0, nil
	}

	var maxID int64
	for _, item := range batch {
		maxID = item.ID
		q.processActivityWithTimeout(ctx, item)
	}

	if _, err := q.DB.ExecContext(ctx, `delete from inbox where id <= ?`, maxID); err != nil {
		return 0, fmt.Errorf("failed to delete processed activities: %w", err)
	}

	return len(batch), nil
}

func (q *Queue) process(ctx context.Context) error {
	t := time.NewTicker(q.Config.ActivitiesBatchDelay)
	defer t.Stop()

	for {
		n, err := q.ProcessBatch(ctx)
		if err != nil {
			return err
		}

		if n < q.Config.ActivitiesBatchSize {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
		}
	}
}

// Process polls the queue of incoming activities and processes them.
func (q *Queue) Process(ctx context.Context) error {
	t := time.NewTicker(q.Config.ActivitiesPollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
			if err := q.process(ctx); err != nil {
				return err
			}
		}
	}
}
