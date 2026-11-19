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
	DBW    *sql.DB
}

// Collect deletes old data.
func (gc *GarbageCollector) Run(ctx context.Context) error {
	now := time.Now()

	if _, err := gc.DBW.ExecContext(ctx, `delete from notesfts where rowid in (select notes.pk from notes left join follows on follows.followed in (notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and notes.object->'to' ? follows.followed) or (notes.cc2 is not null and notes.object->'cc' ? follows.followed) where follows.accepted and notes.inserted < $1 and notes.host != $2 and follows.id is null and not exists (select 1 from bookmarks where bookmarks.note = notes.id) and not exists (select 1 from shares where shares.note = notes.id and exists (select 1 from persons where persons.id = shares.by and persons.host = $2)))`, now.Add(-gc.Config.InvisiblePostsTTL), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove invisible posts: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from notes where id in (select notes.id from notes left join follows on follows.followed in (notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and notes.object->'to' ? follows.followed) or (notes.cc2 is not null and notes.object->'cc' ? follows.followed) where follows.accepted and notes.inserted < $1 and notes.host != $2 and follows.id is null and not exists (select 1 from bookmarks where bookmarks.note = notes.id) and not exists (select 1 from shares where shares.note = notes.id and exists (select 1 from persons where persons.id = shares.by and persons.host = $2)))`, now.Add(-gc.Config.InvisiblePostsTTL), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove invisible posts: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from notesfts where rowid in (select pk from notes where inserted < $1 and author not in (select followed from follows where accepted) and host != $2 and not exists (select 1 from bookmarks where bookmarks.note = notes.id))`, now.Add(-gc.Config.InvisiblePostsTTL), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove posts by authors without followers: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from notes where inserted < $1 and author not in (select followed from follows where accepted) and host != $2 and not exists (select 1 from bookmarks where bookmarks.note = notes.id)`, now.Add(-gc.Config.InvisiblePostsTTL), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove posts by authors without followers: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from notesfts where rowid in (select pk from notes where inserted < $1 and host != $2 and not exists (select 1 from bookmarks where bookmarks.note = notes.id))`, now.Add(-gc.Config.NotesTTL), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from notes where inserted < $1 and host != $2 and not exists (select 1 from bookmarks where bookmarks.note = notes.id)`, now.Add(-gc.Config.NotesTTL), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from shares where "by" not in (select id from persons) or note not in (select id from notes)`); err != nil {
		return fmt.Errorf("failed to remove old shares: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from outbox where inserted < $1 and host != $2`, now.Add(-gc.Config.DeliveryTTL), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from feed where inserted < $1`, now.Add(-gc.Config.FeedTTL)); err != nil {
		return fmt.Errorf("failed to trim feed: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from history where inserted < $1`, now.Add(-gc.Config.HistoryTTL)); err != nil {
		return fmt.Errorf("failed to trim history: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from bookmarks where "by" not in (select id from persons)`); err != nil {
		return fmt.Errorf("failed to remove bookmarks by deleted users: %w", err)
	}

	if _, err := gc.DBW.ExecContext(
		ctx,
		`delete from bookmarks where not exists (
			select 1 from notes
			where
				notes.id = bookmarks.note and
				(
					notes.author = bookmarks.by or
					notes.public or
					exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = bookmarks.by and follows.followed = notes.author and notes.object->'to' ?| array[notes.author, persons.actor->>'followers']) or
					exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = bookmarks.by and follows.followed = notes.author and notes.object->'cc' ?| array[notes.author, persons.actor->>'followers']) or
					notes.object->'to' ? bookmarks.by or
					notes.object->'cc' ? bookmarks.by
				)
		)`,
	); err != nil {
		return fmt.Errorf("failed to invisible bookmarks: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from certificates where not approved and inserted < $1`, now.Add(-gc.Config.CertificateApprovalTimeout)); err != nil {
		return fmt.Errorf("failed to remove timed out certificate approval requests: %w", err)
	}

	if _, err := gc.DBW.ExecContext(ctx, `delete from certificates where expires < now()`); err != nil {
		return fmt.Errorf("failed to remove expired certificates: %w", err)
	}

	return nil
}
