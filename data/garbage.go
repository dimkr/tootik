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

package data

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/dimkr/tootik/cfg"
)

type GarbageCollector struct {
	Domain string
	Config *cfg.Config
	DB     *sql.DB
}

// Collect deletes old data.
func (gc *GarbageCollector) Run(ctx context.Context) error {
	now := time.Now()

	if _, err := gc.DB.ExecContext(ctx, `delete from notesfts where id in (select notes.id from notes left join follows on follows.followed in (notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = follows.followed)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = follows.followed)) where follows.accepted = 1 and notes.inserted < $1 and notes.host != $2 and follows.id is null and not exists (select 1 from bookmarks where bookmarks.note = notesfts.id) and not exists (select 1 from shares where shares.note = notesfts.id and exists (select 1 from persons where persons.id = shares.by and persons.host = $2)))`, now.Add(-gc.Config.InvisiblePostsTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove invisible posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notes where id in (select notes.id from notes left join follows on follows.followed in (notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = follows.followed)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = follows.followed)) where follows.accepted = 1 and notes.inserted < $1 and notes.host != $2 and follows.id is null and not exists (select 1 from bookmarks where bookmarks.note = notes.id) and not exists (select 1 from shares where shares.note = notes.id and exists (select 1 from persons where persons.id = shares.by and persons.host = $2)))`, now.Add(-gc.Config.InvisiblePostsTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove invisible posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notesfts where id in (select id from notes where inserted < $1 and author not in (select followed from follows where accepted = 1) and host != $2 and not exists (select 1 from bookmarks where bookmarks.note = notes.id))`, now.Add(-gc.Config.InvisiblePostsTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove posts by authors without followers: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notes where inserted < $1 and author not in (select followed from follows where accepted = 1) and host != $2 and not exists (select 1 from bookmarks where bookmarks.note = notes.id)`, now.Add(-gc.Config.InvisiblePostsTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove posts by authors without followers: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notesfts where id in (select id from notes where inserted < ? and host != ? and not exists (select 1 from bookmarks where bookmarks.note = notes.id))`, now.Add(-gc.Config.NotesTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notes where inserted < ? and host != ? and not exists (select 1 from bookmarks where bookmarks.note = notes.id)`, now.Add(-gc.Config.NotesTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from hashtags where note not in (select id from notes)`); err != nil {
		return fmt.Errorf("failed to remove old hashtags: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from shares where by not in (select id from persons) or note not in (select id from notes)`); err != nil {
		return fmt.Errorf("failed to remove old shares: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from outbox where inserted < ? and host != ?`, now.Add(-gc.Config.DeliveryTTL).UnixNano(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from persons where updated < ? and ed25519privkey is null and not exists (select 1 from follows where followed = persons.id) and not exists (select 1 from follows where follower = persons.id) and not exists (select 1 from notes where notes.author = persons.id) and not exists (select 1 from shares where shares.by = persons.id)`, now.Add(-gc.Config.ActorTTL).Unix()); err != nil {
		return fmt.Errorf("failed to remove idle actors: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from feed where inserted < ?`, now.Add(-gc.Config.FeedTTL).Unix()); err != nil {
		return fmt.Errorf("failed to trim feed: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from history where inserted < ?`, now.Add(-gc.Config.HistoryTTL).UnixNano()); err != nil {
		return fmt.Errorf("failed to trim history: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from bookmarks where by not in (select id from persons)`); err != nil {
		return fmt.Errorf("failed to remove bookmarks by deleted users: %w", err)
	}

	if _, err := gc.DB.ExecContext(
		ctx,
		`delete from bookmarks where not exists (
			select 1 from notes
			where
				notes.id = bookmarks.note and
				(
					notes.author = bookmarks.by or
					notes.public = 1 or
					exists (select 1 from json_each(notes.object->'$.to') where exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = bookmarks.by and follows.followed = notes.author and (notes.author = value or persons.actor->>'$.followers' = value))) or
					exists (select 1 from json_each(notes.object->'$.cc') where exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = bookmarks.by and follows.followed = notes.author and (notes.author = value or persons.actor->>'$.followers' = value))) or
					exists (select 1 from json_each(notes.object->'$.to') where value = bookmarks.by) or
					exists (select 1 from json_each(notes.object->'$.cc') where value = bookmarks.by)
				)
		)`,
	); err != nil {
		return fmt.Errorf("failed to invisible bookmarks: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from certificates where approved = 0 and inserted < ?`, now.Add(-gc.Config.CertificateApprovalTimeout).Unix()); err != nil {
		return fmt.Errorf("failed to remove timed out certificate approval requests: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from certificates where expires < unixepoch()`); err != nil {
		return fmt.Errorf("failed to remove expired certificates: %w", err)
	}

	return nil
}
