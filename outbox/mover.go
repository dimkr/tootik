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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
)

type Mover struct {
	Domain   string
	DB       *sql.DB
	Resolver ap.Resolver
	Keys     [2]httpsig.Key
	Inbox    ap.Inbox
}

func (m *Mover) updatedMoveTargets(ctx context.Context, prefix string) error {
	rows, err := m.DB.QueryContext(ctx, `select oldid, newid from (select old.id as oldid, new.id as newid, old.updated as oldupdated from persons old join persons new on old.actor->>'$.movedTo' = new.id and not exists (select 1 from json_each(new.actor->'$.alsoKnownAs') where value = old.id) and old.updated > new.updated where old.actor->>'$.movedTo' is not null union all select old.id, old.actor->>'$.movedTo', old.updated from persons old where old.actor->>'$.movedTo' is not null and not exists (select 1 from persons new where new.id = old.actor->>'$.movedTo')) where exists (select 1 from follows where followed = oldid and follower like ? and accepted = 1 and inserted < oldupdated)`, prefix)
	if err != nil {
		return fmt.Errorf("failed to moved actors: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var oldID, newID string
		if err := rows.Scan(&oldID, &newID); err != nil {
			slog.ErrorContext(ctx, "Failed to scan moved actor", "error", err)
			continue
		}

		actor, err := m.Resolver.ResolveID(ctx, m.Keys, newID, 0)
		if err != nil {
			slog.WarnContext(ctx, "Failed to resolve move target", "old", oldID, "new", newID, "error", err)
			continue
		}

		if !actor.AlsoKnownAs.Contains(oldID) {
			slog.WarnContext(ctx, "New account does not point to old account", "new", newID, "old", oldID)
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
			select json(persons.actor), persons.ed25519privkey, old.id, new.id, follows.id, new.id = follows.follower or exists (select 1 from follows where follower = persons.id and followed = new.id) from
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
		var ed25519PrivKeyMultibase string
		var oldID, NewID, oldFollowID string
		var onlyRemove bool
		if err := rows.Scan(&actor, &ed25519PrivKeyMultibase, &oldID, &NewID, &oldFollowID, &onlyRemove); err != nil {
			slog.ErrorContext(ctx, "Failed to scan follow to move", "error", err)
			continue
		}

		ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to decode Ed25519 private key", "error", err)
			continue
		}

		if onlyRemove {
			slog.InfoContext(ctx, "Removing follow of moved actor", "follow", oldFollowID, "old", oldID, "new", NewID)
		} else {
			slog.InfoContext(ctx, "Moving follow", "follow", oldFollowID, "old", oldID, "new", NewID)
			if err := m.Inbox.Follow(ctx, &actor, httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}, NewID); err != nil {
				slog.WarnContext(ctx, "Failed to follow new actor", "follow", oldFollowID, "old", oldID, "new", NewID, "error", err)
				continue
			}
		}
		if err := m.Inbox.Unfollow(ctx, &actor, httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}, oldID, oldFollowID); err != nil {
			slog.WarnContext(ctx, "Failed to unfollow old actor", "follow", oldFollowID, "old", oldID, "new", NewID, "error", err)
		}
	}

	return nil
}
