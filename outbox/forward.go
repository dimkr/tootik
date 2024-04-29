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
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"log/slog"
	"strings"
	"time"
)

func forwardToGroup(ctx context.Context, domain string, log *slog.Logger, tx *sql.Tx, note *ap.Object, activity *ap.Activity, rawActivity json.RawMessage, firstPostID string) (bool, error) {
	var group ap.Actor
	if err := tx.QueryRowContext(
		ctx,
		`
			select actor from
			(
				select persons.actor, 1 as rank
				from persons
				join notes
				on
					notes.object->>'$.audience' = persons.id
				where
					notes.id = $1 and
					persons.host = $2 and
					persons.actor->>'$.type' = 'Group'
				union all
				select persons.actor, 2 as rank
				from persons
				join notes
				on
					exists (select 1 from json_each(notes.object->'$.to') where value = persons.id)
				where
					notes.id = $1 and
					persons.host = $2 and
					persons.actor->>'$.type' = 'Group'
				union all
				select persons.actor, 3 as rank
				from persons
				join notes
				on
					exists (select 1 from json_each(notes.object->'$.cc') where value = persons.id)
				where
					notes.id = $1 and
					persons.host = $2 and
					persons.actor->>'$.type' = 'Group'
				order by rank
				limit 1
			)
		`,
		firstPostID,
		domain,
	).Scan(&group); err != nil && errors.Is(err, sql.ErrNoRows) {
		return false, nil
	} else if err != nil {
		return false, err
	}

	var following int
	if err := tx.QueryRowContext(ctx, `select exists (select 1 from follows where follower = ? and followed = ? and accepted = 1)`, activity.Actor, group.ID).Scan(&following); err != nil {
		return false, err
	}

	if following == 0 {
		return false, nil
	}

	now := time.Now()

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	cc.Add(group.Followers)

	announce := ap.Activity{
		Context:   "https://www.w3.org/ns/activitystreams",
		ID:        fmt.Sprintf("https://%s/announce/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", group.ID, note.ID, now.UnixNano())))),
		Type:      ap.Announce,
		Actor:     group.ID,
		Published: &ap.Time{Time: now},
		To:        to,
		CC:        cc,
		Object:    rawActivity,
	}

	log.Info("Forwarding activity to group followers", "group", group.ID, "announce", announce.ID)

	if _, err := tx.ExecContext(
		ctx,
		`insert into outbox(activity, sender) values(?, ?)`,
		&announce,
		group.ID,
	); err != nil {
		return false, err
	}

	// if this is a new post and we're passing the Create activity to followers, also share the post
	if activity.Type == ap.Create {
		announce.ID += "#share"
		announce.Object = note.ID
		if _, err := tx.ExecContext(
			ctx,
			`insert into outbox(activity, sender) values(?, ?)`,
			&announce,
			group.ID,
		); err != nil {
			return false, err
		}

		if _, err := tx.ExecContext(
			ctx,
			`insert into shares(note, by) values(?, ?)`,
			note.ID,
			group.ID,
		); err != nil {
			return false, err
		}

		if _, err := tx.ExecContext(
			ctx,
			`update notes set object = json_set(object, '$.audience', $1) where id = $2`,
			group.ID,
			note.ID,
		); err != nil {
			return false, err
		}
	}

	return true, nil
}

// ForwardActivity forwards an activity if needed.
// A reply by B in a thread started by A is forwarded to all followers of A.
func ForwardActivity(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, tx *sql.Tx, note *ap.Object, activity *ap.Activity, rawActivity json.RawMessage) error {
	// poll votes don't need to be forwarded
	if note.Name != "" && note.Content == "" {
		return nil
	}

	firstPostID := note.ID
	var threadStarterID string

	if note.InReplyTo != "" {
		var depth int
		if err := tx.QueryRowContext(ctx, `with recursive thread(id, author, parent, depth) as (select notes.id, notes.author, notes.object->>'$.inReplyTo' as parent, 1 as depth from notes where id = $1 union all select notes.id, notes.author, notes.object->>'$.inReplyTo' as parent, t.depth + 1 from thread t join notes on notes.id = t.parent where t.depth <= $2) select id, author, depth from thread order by depth desc limit 1`, note.ID, cfg.MaxForwardingDepth+1).Scan(&firstPostID, &threadStarterID, &depth); err != nil && errors.Is(err, sql.ErrNoRows) {
			log.Debug("Failed to find thread for post", "note", note.ID)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to fetch first post in thread: %w", err)
		}
		if depth > cfg.MaxForwardingDepth {
			log.Debug("Thread exceeds depth limit for forwarding")
			return nil
		}
	}

	if note.IsPublic() {
		forwarded, err := forwardToGroup(ctx, domain, log, tx, note, activity, rawActivity, firstPostID)
		if err != nil {
			return err
		} else if forwarded {
			return nil
		}
	}

	// only replies need to be forwarded
	if note.InReplyTo == "" {
		return nil
	}

	prefix := fmt.Sprintf("https://%s/", domain)
	if !strings.HasPrefix(threadStarterID, prefix) {
		log.Debug("Thread starter is federated")
		return nil
	}

	var shouldForward int
	if err := tx.QueryRowContext(ctx, `select exists (select 1 from notes join persons on persons.id = notes.author and (notes.public = 1 or exists (select 1 from json_each(notes.object->'$.to') where value = persons.actor->>'$.followers') or exists (select 1 from json_each(notes.object->'$.cc') where value = persons.actor->>'$.followers')) where notes.id = ?)`, firstPostID).Scan(&shouldForward); err != nil {
		return err
	}
	if shouldForward == 0 {
		log.Debug("Activity does not need to be forwarded")
		return nil
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO outbox (activity, sender) VALUES(?,?)`,
		string(rawActivity),
		threadStarterID,
	); err != nil {
		return err
	}

	log.Info("Forwarding activity to followers of thread starter", "thread", firstPostID, "starter", threadStarterID)
	return nil
}
