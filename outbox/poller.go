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
	"github.com/dimkr/tootik/dbx"
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
	prefix := fmt.Sprintf("https://%s/%%", p.Domain)

	votes, err := dbx.QueryCollectIgnore[struct {
		PollID string
		Option sql.NullString
		Count  int64
	}](
		ctx,
		p.DB,
		func(err error) bool {
			slog.Warn("Failed to scan poll result", "error", err)
			return true
		},
		`select poll, option, count(case when voter is not null then 1 end) from (select polls.id as poll, votes.object->>'$.name' as option, votes.author as voter from notes polls left join notes votes on votes.object->>'$.inReplyTo' = polls.id and votes.deleted = 0 where polls.object->>'$.type' = 'Question' and polls.id like $1 and polls.deleted = 0 and polls.object->>'$.closed' is null and (votes.object->>'$.name' is not null or votes.id is null) group by poll, option, voter) group by poll, option`,
		prefix,
	)
	if err != nil {
		return err
	}

	voters, err := dbx.QueryCollectIgnore[struct {
		PollID string
		Count  int64
	}](
		ctx,
		p.DB,
		func(err error) bool {
			slog.Warn("Failed to scan poll result", "error", err)
			return true
		},
		`
		select polls.id, count(distinct voters.cid) from
		notes polls
		join notes votes on votes.object->>'$.inReplyTo' = polls.id
		join persons voters on voters.id = votes.author
		where
			polls.object->>'$.type' = 'Question'
			and polls.id like $1
			and polls.deleted = 0
			and votes.deleted = 0
			and polls.object->>'$.closed' is null
			and votes.object->>'$.name' is not null
		group by polls.id
		`,
		prefix,
	)
	if err != nil {
		return err
	}

	results := map[pollResult]int64{}
	polls := map[string]*ap.Object{}
	authors := map[string]*ap.Actor{}
	keys := map[string]ed25519.PrivateKey{}

	for _, vote := range votes {
		if _, ok := polls[vote.PollID]; ok {
			if vote.Option.Valid {
				results[pollResult{PollID: vote.PollID, Option: vote.Option.String}] = vote.Count
			}
			continue
		}

		var obj ap.Object
		var author ap.Actor
		var ed25519PrivKey []byte
		if err := p.DB.QueryRowContext(
			ctx,
			"select json(notes.object), json(persons.actor), persons.ed25519privkey from notes join persons on persons.id = notes.author where notes.id = ? and notes.deleted = 0",
			vote.PollID,
		).Scan(&obj, &author, &ed25519PrivKey); err != nil {
			slog.Warn("Failed to fetch poll", "poll", vote.PollID, "error", err)
			continue
		}

		polls[vote.PollID] = &obj
		if vote.Option.Valid {
			results[pollResult{PollID: vote.PollID, Option: vote.Option.String}] = vote.Count
		}

		authors[vote.PollID] = &author
		keys[vote.PollID] = ed25519.NewKeyFromSeed(ed25519PrivKey)
	}

	changed := make(map[string]bool, len(polls))

	for _, item := range voters {
		if poll, ok := polls[item.PollID]; ok && poll.VotersCount == item.Count {
			changed[item.PollID] = false
		} else if ok {
			poll.VotersCount = item.Count
			changed[item.PollID] = true
		}
	}

	for pollID, poll := range polls {
		if _, ok := changed[pollID]; !ok && poll.VotersCount != 0 {
			poll.VotersCount = 0

			for i := range poll.AnyOf {
				poll.AnyOf[i].Replies.TotalItems = 0
			}

			changed[pollID] = true
		}
	}

	now := ap.Time{Time: time.Now()}

	for pollID, poll := range polls {
		if (poll.EndTime == (ap.Time{}) || now.After(poll.EndTime.Time)) && poll.Closed == (ap.Time{}) {
			poll.Closed = now
			changed[pollID] = true
		}

		if poll.VotersCount == 0 {
			continue
		}

		for i := range poll.AnyOf {
			if count := results[pollResult{PollID: poll.ID, Option: poll.AnyOf[i].Name}]; poll.AnyOf[i].Replies.TotalItems != count {
				poll.AnyOf[i].Replies.TotalItems = count
				changed[pollID] = true
			}
		}
	}

	for pollID, poll := range polls {
		if !changed[pollID] {
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
