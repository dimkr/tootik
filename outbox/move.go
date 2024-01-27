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

package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	"log/slog"
)

type Mover struct {
	Domain   string
	Log      *slog.Logger
	DB       *sql.DB
	Resolver *fed.Resolver
	Actor    *ap.Actor
}

func (m *Mover) updatedMoveTargets(ctx context.Context, prefix string) error {
	rows, err := m.DB.QueryContext(ctx, `select oldid, newid from (select old.id as oldid, new.id as newid, old.updated as oldupdated from persons old join persons new on old.actor->>'movedTo' = new.id and not exists (select 1 from json_each(new.actor->'alsoKnownAs') where value = old.id) and old.updated > new.updated where old.actor->>'movedTo' is not null union select old.id, old.actor->>'movedTo', old.updated from persons old where old.actor->>'movedTo' is not null and not exists (select 1 from persons new where new.id = old.actor->>'movedTo')) where exists (select 1 from follows where followed = oldid and follower like ? and inserted < oldupdated)`, prefix)
	if err != nil {
		return fmt.Errorf("failed to moved actors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var oldID, newID string
		if err := rows.Scan(&oldID, &newID); err != nil {
			m.Log.Error("Failed to scan moved actor", "error", err)
			continue
		}

		actor, err := m.Resolver.Resolve(ctx, m.Log, m.DB, m.Actor, newID, false)
		if err != nil {
			m.Log.Warn("Failed to resolve move target", "old", oldID, "new", newID, "error", err)
			continue
		}

		if !actor.AlsoKnownAs.Contains(oldID) {
			m.Log.Warn("New account does not point to old account", "new", newID, "old", oldID)
		}
	}

	return nil
}

func (m *Mover) Run(ctx context.Context) error {
	prefix := fmt.Sprintf("https://%s/%%", m.Domain)

	// updated new actor if old actor specifies movedTo but new actor doesn't specify old actor in alsoKnownAs
	if err := m.updatedMoveTargets(ctx, prefix); err != nil {
		return err
	}

	rows, err := m.DB.QueryContext(ctx, `select persons.actor, old.id, new.id, follows.id from persons old join persons new on old.actor->>'movedTo' = new.id and exists (select 1 from json_each(new.actor->'alsoKnownAs') where value = old.id) join follows on follows.followed = old.id and follows.inserted < old.updated join persons on persons.id = follows.follower where old.actor->>'movedTo' is not null and follows.follower like ?`, prefix)
	if err != nil {
		return fmt.Errorf("failed to fetch follows to move: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var actor ap.Actor
		var oldID, newID, oldFollowID string
		if err := rows.Scan(&actor, &oldID, &newID, &oldFollowID); err != nil {
			m.Log.Error("Failed to scan follow to move", "error", err)
			continue
		}

		m.Log.Info("Moving follow", "follow", oldFollowID, "old", oldID, "new", newID)
		if err := Follow(ctx, m.Domain, &actor, newID, m.DB); err != nil {
			m.Log.Warn("Failed to follow new actor", "follow", oldFollowID, "old", oldID, "new", newID, "error", err)
			continue
		}
		if err := Unfollow(ctx, m.Domain, m.Log, m.DB, &actor, oldID, oldFollowID); err != nil {
			m.Log.Warn("Failed to unfollow old actor", "follow", oldFollowID, "old", oldID, "new", newID, "error", err)
		}
	}

	return nil
}
