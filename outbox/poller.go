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

	rows, err := dbx.QueryCollectIgnore[struct {
		PollID      string
		Option      sql.NullString
		OptionCount int64
		VotersCount int64
	}](
		ctx,
		p.DB,
		func(err error) bool {
			slog.Warn("Failed to scan poll result", "error", err)
			return true
		},
		`
		with options as (
			select notes.id as poll, json_each.value->>'$.name' as option
			from notes, json_each(notes.object->'$.anyOf')
			where
				notes.object->>'$.type' = 'Question'
				and notes.id like $1
				and notes.deleted = 0
				and notes.object->>'$.closed' is null
		)
		select options.poll, options.option, coalesce(option_counts.count, 0), coalesce(voter_counts.count, 0)
		from options
		left join (
			select options.poll, options.option, count(distinct voters.cid) as count
			from options
			join notes votes on votes.object->>'$.inReplyTo' = options.poll and votes.object->>'$.name' = options.option
			join persons voters on voters.id = votes.author
			where votes.deleted = 0
			group by options.poll, options.option
		) option_counts on option_counts.poll = options.poll and option_counts.option = options.option
		left join (
			select options.poll, count(distinct voters.cid) as count
			from options
			join notes votes on votes.object->>'$.inReplyTo' = options.poll and votes.object->>'$.name' = options.option
			join persons voters on voters.id = votes.author
			where votes.deleted = 0
			group by options.poll
		) voter_counts on voter_counts.poll = options.poll
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
	voterCounts := map[string]int64{}

	for _, r := range rows {
		voterCounts[r.PollID] = r.VotersCount

		if _, ok := polls[r.PollID]; ok {
			if r.Option.Valid {
				results[pollResult{PollID: r.PollID, Option: r.Option.String}] = r.OptionCount
			}
			continue
		}

		var obj ap.Object
		var author ap.Actor
		var ed25519PrivKey []byte
		if err := p.DB.QueryRowContext(
			ctx,
			"select json(notes.object), json(persons.actor), persons.ed25519privkey from notes join persons on persons.id = notes.author where notes.id = ? and notes.deleted = 0",
			r.PollID,
		).Scan(&obj, &author, &ed25519PrivKey); err != nil {
			slog.Warn("Failed to fetch poll", "poll", r.PollID, "error", err)
			continue
		}

		polls[r.PollID] = &obj
		if r.Option.Valid {
			results[pollResult{PollID: r.PollID, Option: r.Option.String}] = r.OptionCount
		}

		authors[r.PollID] = &author
		keys[r.PollID] = ed25519.NewKeyFromSeed(ed25519PrivKey)
	}

	changed := make(map[string]bool, len(polls))

	for pollID, count := range voterCounts {
		if poll, ok := polls[pollID]; ok && poll.VotersCount == count {
			changed[pollID] = false
		} else if ok {
			poll.VotersCount = count
			if count == 0 {
				for i := range poll.AnyOf {
					poll.AnyOf[i].Replies.TotalItems = 0
				}
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
