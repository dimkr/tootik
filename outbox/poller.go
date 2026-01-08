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

package outbox

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

type Poller struct {
	Domain string
	DB     *sql.DB
	Inbox  ap.Inbox
}

type pollResult struct {
	PollID, Option string
}

func (p *Poller) Run(ctx context.Context) error {
	rows, err := p.DB.QueryContext(ctx, `select poll, option, count(case when voter is not null then 1 end) from (select polls.id as poll, votes.object->>'$.name' as option, votes.author as voter from notes polls left join notes votes on votes.object->>'$.inReplyTo' = polls.id and votes.deleted = 0 where polls.object->>'$.type' = 'Question' and polls.id like $1 and polls.deleted = 0 and polls.object->>'$.closed' is null and (votes.object->>'$.name' is not null or votes.id is null) group by poll, option, voter) group by poll, option`, fmt.Sprintf("https://%s/%%", p.Domain))
	if err != nil {
		return err
	}
	defer rows.Close()

	votes := map[pollResult]int64{}
	polls := map[string]*ap.Object{}
	authors := map[string]*ap.Actor{}
	keys := map[string]ed25519.PrivateKey{}

	for rows.Next() {
		var pollID string
		var option sql.NullString
		var count int64
		if err := rows.Scan(&pollID, &option, &count); err != nil {
			slog.Warn("Failed to scan poll result", "error", err)
			continue
		}

		if _, ok := polls[pollID]; ok {
			if option.Valid {
				votes[pollResult{PollID: pollID, Option: option.String}] = count
			}
			continue
		}

		var obj ap.Object
		var author ap.Actor
		var ed25519PrivKey []byte
		if err := p.DB.QueryRowContext(ctx, "select json(notes.object), json(persons.actor), persons.ed25519privkey from notes join persons on persons.id = notes.author where notes.id = ? and notes.deleted = 0", pollID).Scan(&obj, &author, &ed25519PrivKey); err != nil {
			slog.Warn("Failed to fetch poll", "poll", pollID, "error", err)
			continue
		}

		polls[pollID] = &obj
		if option.Valid {
			votes[pollResult{PollID: pollID, Option: option.String}] = count
		}

		authors[pollID] = &author
		keys[pollID] = ed25519.NewKeyFromSeed(ed25519PrivKey)
	}
	rows.Close()

	now := ap.Time{Time: time.Now()}

	for pollID, poll := range polls {
		changed := false

		poll.VotersCount = 0

		for i := range poll.AnyOf {
			count, ok := votes[pollResult{PollID: poll.ID, Option: poll.AnyOf[i].Name}]
			if !ok {
				changed = changed || poll.AnyOf[i].Replies.TotalItems > 0
				poll.AnyOf[i].Replies.TotalItems = 0
				continue
			}

			changed = changed || poll.AnyOf[i].Replies.TotalItems != count
			poll.AnyOf[i].Replies.TotalItems = count
			poll.VotersCount += count
		}

		if poll.EndTime == (ap.Time{}) || now.After(poll.EndTime.Time) {
			poll.Closed = now
			changed = true
		}

		if !changed {
			continue
		}

		poll.Updated = now

		slog.Info("Updating poll results", "poll", poll.ID)

		author := authors[pollID]
		if err := p.Inbox.UpdateNote(ctx, author, httpsig.Key{ID: author.AssertionMethod[0].ID, PrivateKey: keys[pollID]}, poll); err != nil {
			slog.Warn("Failed to update poll results", "poll", poll.ID, "error", err)
		}
	}

	return nil
}
