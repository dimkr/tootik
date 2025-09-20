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
	"log/slog"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/logcontext"
)

type Queue struct {
	Config   *cfg.Config
	DB       *sql.DB
	Inbox    ap.Inbox
	Resolver ap.Resolver
	Keys     [2]httpsig.Key
}

type batchItem struct {
	Activity    *ap.Activity
	RawActivity string
	Sender      *ap.Actor
	Shared      bool
}

func (q *Queue) processActivityWithTimeout(parent context.Context, sender *ap.Actor, activity *ap.Activity, rawActivity string, shared bool) {
	ctx, cancel := context.WithTimeout(parent, q.Config.ActivityProcessingTimeout)
	defer cancel()

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		slog.WarnContext(ctx, "Failed to start transaction", "error", err)
		return
	}
	defer tx.Rollback()

	if _, err := q.Resolver.ResolveID(ctx, q.Keys, activity.Actor, 0); err != nil {
		slog.WarnContext(ctx, "Failed to resolve actor", "error", err)
	} else if err := q.Inbox.ProcessActivity(ctx, tx, sender, activity, rawActivity, 1, shared); err != nil {
		slog.WarnContext(ctx, "Failed to process activity", "error", err)
	} else if err := tx.Commit(); err != nil {
		slog.WarnContext(ctx, "Failed to commit changes", "error", err)
	}
}

// ProcessBatch processes one batch of incoming activites in the queue.
func (q *Queue) ProcessBatch(ctx context.Context) (int, error) {
	slog.DebugContext(ctx, "Polling activities queue")

	rows, err := q.DB.QueryContext(ctx, `select inbox.id, json(persons.actor), json(inbox.activity), inbox.raw, inbox.raw->>'$.type' = 'Announce' as shared from (select * from inbox limit -1 offset case when (select count(*) from inbox) >= $1 then $1/10 else 0 end) inbox left join persons on persons.id = inbox.sender order by inbox.id limit $2`, q.Config.MaxActivitiesQueueSize, q.Config.ActivitiesBatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch activities to process: %w", err)
	}
	defer rows.Close()

	batch := make([]batchItem, 0, q.Config.ActivitiesBatchSize)
	var maxID int64
	var rowsCount int

	for rows.Next() {
		rowsCount += 1

		var id int64
		var activityString string
		var activity ap.Activity
		var sender sql.Null[ap.Actor]
		var shared bool
		if err := rows.Scan(&id, &sender, &activity, &activityString, &shared); err != nil {
			slog.ErrorContext(ctx, "Failed to scan activity", "error", err)
			continue
		}

		maxID = id

		if !sender.Valid {
			slog.WarnContext(ctx, "Sender is unknown", "id", id)
			continue
		}

		batch = append(batch, batchItem{
			Activity:    &activity,
			RawActivity: activityString,
			Sender:      &sender.V,
			Shared:      shared,
		})
	}
	rows.Close()

	if len(batch) == 0 {
		return 0, nil
	}

	for _, item := range batch {
		q.processActivityWithTimeout(logcontext.New(ctx, "activity", item.Activity), item.Sender, item.Activity, item.RawActivity, item.Shared)
	}

	if _, err := q.DB.ExecContext(ctx, `delete from inbox where id <= ?`, maxID); err != nil {
		return 0, fmt.Errorf("failed to delete processed activities: %w", err)
	}

	return rowsCount, nil
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
