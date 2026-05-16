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

func (p *Poller) Run(ctx context.Context) error {
	rows, err := dbx.QueryCollectIgnore[struct {
		PollID         string
		Option         sql.NullString
		OptionCount    int64
		VotersCount    int64
		Object         ap.Object
		Actor          ap.Actor
		ED25519PrivKey []byte
	}](
		ctx,
		p.DB,
		func(err error) bool {
			slog.Warn("Failed to scan poll result", "error", err)
			return true
		},
		`
		with polls as (
			select notes.id, notes.object, persons.actor as author, persons.ed25519privkey
			from notes
			join persons on persons.id = notes.author
			where
				notes.object->>'$.type' = 'Question'
				and notes.id like $1
				and notes.deleted = 0
				and notes.object->>'$.closed' is null
		)
		select
			polls.id,
			anyof.value->>'$.name',
			coalesce(option_counts.count, 0),
			coalesce(voter_counts.count, 0),
			json(polls.object),
			json(polls.author),
			polls.ed25519privkey
		from polls
		join json_each(polls.object->'$.anyOf') as anyof
		left join (
			select polls.id as poll, votes.object->>'$.name' as name, count(distinct voters.cid) as count
			from notes votes
			join polls on votes.object->>'$.inReplyTo' = polls.id
			join persons voters on voters.id = votes.author
			where votes.deleted = 0
			group by poll, name
		) option_counts on option_counts.poll = polls.id and option_counts.name = anyof.value->>'$.name'
		left join (
			select polls.id as poll, count(distinct voters.cid) as count
			from notes votes
			join polls on votes.object->>'$.inReplyTo' = polls.id
			join persons voters on voters.id = votes.author
			where votes.deleted = 0
			group by poll
		) voter_counts on voter_counts.poll = polls.id
		`,
		fmt.Sprintf("https://%s/%%", p.Domain),
	)
	if err != nil {
		return err
	}

	type poll struct {
		ap.Object

		Author             ap.Actor
		Key                ed25519.PrivateKey
		CurrentVotersCount int64
		CurrentVotes       map[string]int64
	}
	polls := map[string]*poll{}

	for _, row := range rows {
		info, ok := polls[row.PollID]
		if !ok {
			info = &poll{
				Object:             row.Object,
				Author:             row.Actor,
				Key:                ed25519.NewKeyFromSeed(row.ED25519PrivKey),
				CurrentVotersCount: row.VotersCount,
				CurrentVotes:       make(map[string]int64, len(row.Object.AnyOf)),
			}
			polls[row.PollID] = info
		}

		if row.Option.Valid {
			info.CurrentVotes[row.Option.String] = row.OptionCount
		}
	}

	now := ap.Time{Time: time.Now()}

	for pollID, poll := range polls {
		changed := false

		if poll.VotersCount != poll.CurrentVotersCount {
			poll.VotersCount = poll.CurrentVotersCount
			changed = true
		}

		if poll.EndTime == (ap.Time{}) || now.After(poll.EndTime.Time) {
			poll.Closed = now
			changed = true
		}

		for i := range poll.AnyOf {
			if count := poll.CurrentVotes[poll.AnyOf[i].Name]; poll.AnyOf[i].Replies.TotalItems != count {
				poll.AnyOf[i].Replies.TotalItems = count
				changed = true
			}
		}

		if !changed {
			continue
		}

		poll.Updated = now

		slog.Info("Updating poll results", "poll", poll.ID)

		if err := p.Inbox.UpdateNote(
			ctx,
			&poll.Author,
			httpsig.Key{
				ID:         poll.Author.AssertionMethod[0].ID,
				PrivateKey: poll.Key,
			},
			&poll.Object,
		); err != nil {
			slog.Warn("Failed to update poll results", "poll", pollID, "error", err)
		}
	}

	return nil
}
