/*
Copyright 2023, 2024 Dima Krasner

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
	"github.com/dimkr/tootik/cfg"
	"time"
)

type GarbageCollector struct {
	Domain string
	Config *cfg.Config
	DB     *sql.DB
}

// Collect deletes old data.
func (gc *GarbageCollector) Run(ctx context.Context) error {
	now := time.Now()

	if _, err := gc.DB.ExecContext(ctx, `delete from notesfts where id in (select notes.id from notes left join follows on follows.followed in (notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.followed)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.followed)) where follows.accepted = 1 and notes.inserted < unixepoch()-60*60*24 and notes.host != ? and follows.id is null)`, gc.Domain); err != nil {
		return fmt.Errorf("failed to remove invisible posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notes where id in (select notes.id from notes left join follows on follows.followed in (notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.followed)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.followed)) where follows.accepted = 1 and notes.inserted < unixepoch()-60*60*24 and notes.host != ? and follows.id is null)`, gc.Domain); err != nil {
		return fmt.Errorf("failed to remove invisible posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notesfts where id in (select id from notes where inserted < unixepoch()-60*60*24*7 and author not in (select followed from follows where accepted = 1) and host != ?)`, gc.Domain); err != nil {
		return fmt.Errorf("failed to remove posts by authors without followers: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notes where inserted < unixepoch()-60*60*24*7 and author not in (select followed from follows where accepted = 1) and host != ?`, gc.Domain); err != nil {
		return fmt.Errorf("failed to remove posts by authors without followers: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notesfts where id in (select id from notes where inserted < ? and host != ?)`, now.Add(-gc.Config.NotesTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from notes where inserted < ? and host != ?`, now.Add(-gc.Config.NotesTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from hashtags where not exists (select 1 from notes where notes.id = hashtags.note)`); err != nil {
		return fmt.Errorf("failed to remove old hashtags: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from shares where not exists (select 1 from persons where persons.id = shares.by) or (inserted < ? and not exists (select 1 from notes where notes.id = shares.note))`, now.Add(-gc.Config.SharesTTL).Unix()); err != nil {
		return fmt.Errorf("failed to remove old shares: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from outbox where inserted < ? and host != ?`, now.Add(-gc.Config.DeliveryTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove old posts: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from follows where accepted = 0 and inserted < ?`, now.Add(-gc.Config.FollowAcceptTimeout).Unix()); err != nil {
		return fmt.Errorf("failed to remove failed follow requests: %w", err)
	}

	if _, err := gc.DB.ExecContext(ctx, `delete from persons where updated < ? and host != ? and not exists (select 1 from follows where followed = persons.id) and not exists (select 1 from notes where notes.author = persons.id) and not exists (select 1 from shares where shares.by = persons.id)`, now.Add(-gc.Config.ActorTTL).Unix(), gc.Domain); err != nil {
		return fmt.Errorf("failed to remove idle actors: %w", err)
	}

	return nil
}
