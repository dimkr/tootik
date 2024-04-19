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
	"log/slog"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
)

// ForwardActivity forwards an activity if needed.
// A reply by B in a thread started by A is forwarded to all followers of A.
func ForwardActivity(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, tx *sql.Tx, note *ap.Object, rawActivity []byte) error {
	// only replies need to be forwarded
	if note.InReplyTo == "" {
		return nil
	}

	// poll votes don't need to be forwarded
	if note.Name != "" && note.Content == "" {
		return nil
	}

	var firstPostID, threadStarterID string
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

	fmt.Println(firstPostID)
	fmt.Println(threadStarterID)

	var group ap.Actor
	if err := tx.QueryRowContext(
		ctx,
		`
			select persons.actor
			from persons
			join notes
			on
				notes.object->>'$.audience' = persons.id or
				exists (select 1 from json_each(notes.object->'$.to') where value = persons.id) or
				exists (select 1 from json_each(notes.object->'$.cc') where value = persons.id)
			where
				notes.id = $1 and
				persons.host = $2 and
				persons.actor->>'$.type' = 'Group'
			limit 1
		`,
		firstPostID,
		domain,
	).Scan(&group); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	} else if err == nil {
		// set the audience property on the inner object
		var activity map[string]any
		if err := json.Unmarshal(rawActivity, &activity); err != nil {
			return err
		}

		object, ok := activity["object"]
		if !ok {
			return errors.New("no object")
		}

		m, ok := object.(map[string]any)
		if !ok {
			return errors.New("invalid object")
		}

		m["audience"] = group.ID

		now := time.Now()

		to := ap.Audience{}
		to.Add(group.Followers)

		announce := ap.Activity{
			Context:   "https://www.w3.org/ns/activitystreams",
			ID:        fmt.Sprintf("https://%s/announce/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%d", group.ID, now.UnixNano())))),
			Type:      ap.Announce,
			Actor:     group.ID,
			Published: ap.Time{Time: now},
			To:        to,
			Object:    &activity,
		}

		log.Info("Forwarding activity to group followers", "group", group.ID, "announce", announce.ID)

		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
			&announce,
			group.ID,
		); err != nil {
			return err
		}

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
