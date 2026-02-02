/*
Copyright 2025, 2026 Dima Krasner

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
	"crypto/ed25519"
	"database/sql"
	"log/slog"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/dbx"
	"github.com/dimkr/tootik/httpsig"
)

const batchSize = 512

type Deleter struct {
	DB    *sql.DB
	Inbox ap.Inbox
}

func (d *Deleter) undoShares(ctx context.Context) (bool, error) {
	rows, err := dbx.QueryCollectRows[struct {
		Sharer         ap.Actor
		Ed25519PrivKey []byte
		Share          ap.Activity
	}](
		ctx,
		d.DB,
		`
		select json(persons.actor), persons.ed25519privkey, json(outbox.activity) from persons
		join shares on shares.by = persons.id
		join outbox on outbox.activity->>'$.actor' = shares.by and outbox.activity->>'$.object' = shares.note
		where
			persons.ttl is not null and
			shares.inserted <= unixepoch() - (persons.ttl * 24 * 60 * 60) and
			outbox.activity->>'$.type' = 'Announce'
		order by shares.inserted
		limit ?
		`,
		batchSize,
	)
	if err != nil {
		return false, err
	}

	count := 0
	for _, row := range rows {
		if err := d.Inbox.Undo(
			ctx,
			&row.Sharer,
			httpsig.Key{
				ID:         row.Sharer.AssertionMethod[0].ID,
				PrivateKey: ed25519.NewKeyFromSeed(row.Ed25519PrivKey),
			},
			&row.Share,
		); err != nil {
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
	rows, err := dbx.QueryCollectRows[struct {
		Author         ap.Actor
		Ed25519PrivKey []byte
		Note           ap.Object
	}](
		ctx,
		d.DB,
		`
		select json(persons.actor), persons.ed25519privkey, json(notes.object) from persons
		join notes on notes.author = persons.id
		where
			persons.ttl is not null and
			notes.inserted <= unixepoch() - (persons.ttl * 24 * 60 * 60) and
			notes.deleted = 0 and
			not exists (select 1 from bookmarks where bookmarks.by = persons.id and bookmarks.note = notes.id)
		order by notes.inserted
		limit ?
		`,
		batchSize,
	)
	if err != nil {
		return false, err
	}

	count := 0
	for _, row := range rows {
		if err := d.Inbox.Delete(
			ctx,
			&row.Author,
			httpsig.Key{
				ID:         row.Author.AssertionMethod[0].ID,
				PrivateKey: ed25519.NewKeyFromSeed(row.Ed25519PrivKey),
			},
			&row.Note,
		); err != nil {
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
