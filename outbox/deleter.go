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
	"log/slog"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
)

const batchSize = 512

type Deleter struct {
	Domain string
	Config *cfg.Config
	DB     *sql.DB
}

func (d *Deleter) undoShares(ctx context.Context) (bool, error) {
	rows, err := d.DB.QueryContext(
		ctx,
		`
		select outbox.activity from persons
		join shares on shares.by = persons.id
		join outbox on outbox.activity->>'$.actor' = shares.by and outbox.activity->>'$.object' = shares.note
		where
			persons.ttl is not null and
			shares.inserted < unixepoch() - (persons.ttl * 24 * 60 * 60) and
			outbox.activity->>'$.type' = 'Announce'
		order by shares.inserted
		limit ?
		`,
		batchSize,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var share ap.Activity
		if err := rows.Scan(&share); err != nil {
			return false, err
		}

		if err := Undo(ctx, d.Domain, d.DB, &share); err != nil {
			return false, err
		}

		count++
	}

	if count > 0 {
		slog.Info("Removed old shared posts", "count", count)
		return true, nil
	}

	return false, nil
}

func (d *Deleter) deletePosts(ctx context.Context) (bool, error) {
	rows, err := d.DB.QueryContext(
		ctx,
		`
		select notes.object from persons
		join notes on notes.author = persons.id
		where
			persons.ttl is not null and
			notes.inserted < unixepoch() - (persons.ttl * 24 * 60 * 60) and
			not exists (select 1 from bookmarks where bookmarks.by = persons.id and bookmarks.note = notes.id)
		order by notes.inserted
		limit ?
		`,
		batchSize,
	)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var note ap.Object
		if err := rows.Scan(&note); err != nil {
			return false, err
		}

		if err := Delete(ctx, d.Domain, d.Config, d.DB, &note); err != nil {
			return false, err
		}

		count++
	}

	if count > 0 {
		slog.Info("Deleted old posts", "count", count)
		return true, nil
	}

	return false, nil
}

func (d *Deleter) Run(ctx context.Context) error {
	for {
		if again, err := d.deletePosts(ctx); err != nil {
			return err
		} else if !again {
			break
		}
	}

	for {
		if again, err := d.undoShares(ctx); err != nil {
			return err
		} else if !again {
			return nil
		}
	}
}
