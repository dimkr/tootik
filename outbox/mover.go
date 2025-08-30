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
	"fmt"
	"log/slog"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

type Mover struct {
	DB       *sql.DB
	Domain   string
	Inbox    ap.Inbox
	Resolver ap.Resolver
	Keys     [2]httpsig.Key
}

func (m *Mover) updatedMoveTargets(ctx context.Context, prefix string) error {
	rows, err := m.DB.QueryContext(ctx, `select oldid, newid from (select old.id as oldid, new.id as newid, old.updated as oldupdated from persons old join persons new on old.actor->>'$.movedTo' = new.id and not exists (select 1 from json_each(new.actor->'$.alsoKnownAs') where value = old.id) and old.updated > new.updated where old.actor->>'$.movedTo' is not null union all select old.id, old.actor->>'$.movedTo', old.updated from persons old where old.actor->>'$.movedTo' is not null and not exists (select 1 from persons new where new.id = old.actor->>'$.movedTo')) where exists (select 1 from follows where followed = oldid and follower like ? and accepted = 1 and inserted < oldupdated)`, prefix)
	if err != nil {
		return fmt.Errorf("failed to moved actors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var oldID, NewID string
		if err := rows.Scan(&oldID, &NewID); err != nil {
			slog.Error("Failed to scan moved actor", "error", err)
			continue
		}

		actor, err := m.Resolver.ResolveID(ctx, m.Keys, NewID, 0)
		if err != nil {
			slog.Warn("Failed to resolve move target", "old", oldID, "new", NewID, "error", err)
			continue
		}

		if !actor.AlsoKnownAs.Contains(oldID) {
			slog.Warn("New account does not point to old account", "new", NewID, "old", oldID)
		}
	}

	return nil
}

// Run makes users who follow a moved account follow the target account instead.
// It queues two activites for delivery: an Unfollow activity for the old account and a Follow activity for the new one.
func (m *Mover) Run(ctx context.Context) error {
	prefix := fmt.Sprintf("https://%s/%%", m.Domain)

	// updated new actor if old actor specifies movedTo but new actor doesn't specify old actor in alsoKnownAs
	if err := m.updatedMoveTargets(ctx, prefix); err != nil {
		return err
	}

	rows, err := m.DB.QueryContext(
		ctx,
		`
			select json(persons.actor), old.id, new.id, follows.id, new.id = follows.follower or exists (select 1 from follows where follower = persons.id and followed = new.id) from
			persons old
			join
			persons new
			on
				old.actor->>'$.movedTo' = new.id and
				exists (select 1 from json_each(new.actor->'$.alsoKnownAs') where value = old.id)
			join follows
			on
				follows.followed = old.id
			join persons
			on
				persons.id = follows.follower
			where
				old.actor->>'$.movedTo' is not null and
				follows.follower like ? and
				follows.accepted = 1
		`,
		prefix,
	)
	if err != nil {
		return fmt.Errorf("failed to fetch follows to move: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var actor ap.Actor
		var oldID, NewID, oldFollowID string
		var onlyRemove bool
		if err := rows.Scan(&actor, &oldID, &NewID, &oldFollowID, &onlyRemove); err != nil {
			slog.Error("Failed to scan follow to move", "error", err)
			continue
		}

		if onlyRemove {
			slog.Info("Removing follow of moved actor", "follow", oldFollowID, "old", oldID, "new", NewID)
		} else {
			slog.Info("Moving follow", "follow", oldFollowID, "old", oldID, "new", NewID)
			if err := m.Inbox.Follow(ctx, &actor, NewID, m.DB); err != nil {
				slog.Warn("Failed to follow new actor", "follow", oldFollowID, "old", oldID, "new", NewID, "error", err)
				continue
			}
		}
		if err := m.Inbox.Unfollow(ctx, m.DB, &actor, oldID, oldFollowID); err != nil {
			slog.Warn("Failed to unfollow old actor", "follow", oldFollowID, "old", oldID, "new", NewID, "error", err)
		}
	}

	return nil
}
