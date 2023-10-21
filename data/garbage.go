/*
Copyright 2023 Dima Krasner

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

const (
	notesTTL    = time.Hour * 24 * 30
	deliveryTTL = time.Hour * 24 * 7
)

func CollectGarbage(ctx context.Context, db *sql.DB) error {
	now := time.Now()

	prefix := fmt.Sprintf("https://%s/%%", cfg.Domain)

	if _, err := db.ExecContext(ctx, `delete from notes where id in (select notes.id from notes left join follows on follows.followed in (notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.followed)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.followed)) where follows.accepted = 1 and notes.inserted < unixepoch()-60*60*24 and notes.author not like ? and follows.id is null)`, prefix); err != nil {
		return fmt.Errorf("Failed to remove invisible posts: %w", err)
	}

	if _, err := db.ExecContext(ctx, `delete from notes where inserted < unixepoch()-60*60*24*7 and author not in (select followed from follows where accepted = 1) and author not like ?`, prefix); err != nil {
		return fmt.Errorf("Failed to remove posts by authors without followers: %w", err)
	}

	if _, err := db.ExecContext(ctx, `delete from notes where inserted < ? and author not like ?`, now.Add(-notesTTL).Unix(), prefix); err != nil {
		return fmt.Errorf("Failed to remove old posts: %w", err)
	}

	if _, err := db.ExecContext(ctx, `delete from hashtags where note in (select distinct hashtags.note from hashtags left join notes on notes.id = hashtags.note where notes.id is null)`); err != nil {
		return fmt.Errorf("Failed to remove old hashtags: %w", err)
	}

	if _, err := db.ExecContext(ctx, `delete from outbox where inserted < ?`, now.Add(-deliveryTTL).Unix()); err != nil {
		return fmt.Errorf("Failed to remove old posts: %w", err)
	}

	if _, err := db.ExecContext(ctx, `delete from follows where accepted = 0 and inserted < unixepoch()-60*60*24*2`); err != nil {
		return fmt.Errorf("Failed to remove failed follow requests: %w", err)
	}

	return nil
}
