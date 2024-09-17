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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"log/slog"
	"strings"
)

func forwardToGroup[T ap.RawActivity](
	ctx context.Context,
	domain string,
	tx *sql.Tx,
	note *ap.Object,
	activity *ap.Activity,
	rawActivity T,
	firstPostID string,
) (bool, error) {
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
					notes.object->>'$.audience' is null and
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
					notes.object->>'$.audience' is null and
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
	if err := tx.QueryRowContext(
		ctx,
		`select exists (select 1 from follows where follower = ? and followed = ? and accepted = 1)`,
		note.AttributedTo,
		group.ID,
	).Scan(&following); err != nil {
		return true, err
	}

	if following == 0 {
		return true, nil
	}

	slog.Info("Forwarding post to group followers", "activity", activity.ID, "note", note.ID, "group", group.ID)

	if _, err := tx.ExecContext(
		ctx,
		`insert into outbox(activity, sender) values(?, ?)`,
		rawActivity,
		group.ID,
	); err != nil {
		return true, err
	}

	if activity.Type != ap.Create {
		return true, nil
	}

	// if this is a new post and we're passing the Create activity to followers, also share the post
	if err := Announce(ctx, domain, tx, &group, note); err != nil {
		return true, err
	}

	if _, err := tx.ExecContext(
		ctx,
		`update notes set object = json_set(object, '$.audience', $1) where id = $2`,
		group.ID,
		note.ID,
	); err != nil {
		return true, err
	}

	return true, nil
}

// ForwardActivity forwards an activity if needed.
// A reply by B in a thread started by A is forwarded to all followers of A.
// A post by a follower of a local group, which mentions the group or replies to a post in the group, is forwarded to
// followers of the group.
func ForwardActivity[T ap.RawActivity](
	ctx context.Context,
	domain string,
	cfg *cfg.Config,
	tx *sql.Tx,
	note *ap.Object,
	activity *ap.Activity,
	rawActivity T,
) error {
	// poll votes don't need to be forwarded
	if note.Name != "" && note.Content == "" {
		return nil
	}

	firstPostID := note.ID
	var threadStarterID string

	if note.InReplyTo != "" {
		var depth int
		if err := tx.QueryRowContext(
			ctx,
			`with recursive thread(id, author, parent, depth) as (select notes.id, notes.author, notes.object->>'$.inReplyTo' as parent, 1 as depth from notes where id = $1 union all select notes.id, notes.author, notes.object->>'$.inReplyTo' as parent, t.depth + 1 from thread t join notes on notes.id = t.parent where t.depth <= $2) select id, author, depth from thread order by depth desc limit 1`,
			note.ID,
			cfg.MaxForwardingDepth+1,
		).Scan(&firstPostID, &threadStarterID, &depth); err != nil && errors.Is(err, sql.ErrNoRows) {
			slog.Debug("Failed to find thread for post", "activity", activity.ID, "note", note.ID)
			return nil
		} else if err != nil {
			return fmt.Errorf("failed to fetch first post in thread: %w", err)
		}
		if depth > cfg.MaxForwardingDepth {
			slog.Debug("Thread exceeds depth limit for forwarding", "activity", activity.ID, "note", note.ID)
			return nil
		}
	}

	if note.IsPublic() {
		if groupThread, err := forwardToGroup(ctx, domain, tx, note, activity, rawActivity, firstPostID); err != nil {
			return err
		} else if groupThread {
			return nil
		}
	}

	// only replies need to be forwarded
	if note.InReplyTo == "" {
		return nil
	}

	prefix := fmt.Sprintf("https://%s/", domain)
	if !strings.HasPrefix(threadStarterID, prefix) {
		slog.Debug("Thread starter is federated", "activity", activity.ID, "note", note.ID)
		return nil
	}

	var shouldForward int
	if err := tx.QueryRowContext(
		ctx,
		`select exists (select 1 from notes join persons on persons.id = notes.author and (notes.public = 1 or exists (select 1 from json_each(notes.object->'$.to') where value = persons.actor->>'$.followers') or exists (select 1 from json_each(notes.object->'$.cc') where value = persons.actor->>'$.followers')) where notes.id = ?)`,
		firstPostID,
	).Scan(&shouldForward); err != nil {
		return err
	}
	if shouldForward == 0 {
		slog.Debug("Activity does not need to be forwarded", "activity", activity.ID, "note", note.ID)
		return nil
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO outbox (activity, sender) VALUES(?,?)`,
		rawActivity,
		threadStarterID,
	); err != nil {
		return err
	}

	slog.Info("Forwarding activity to followers of thread starter", "activity", activity.ID, "note", note.ID, "thread", firstPostID, "starter", threadStarterID)
	return nil
}
