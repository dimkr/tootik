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
		WITH polls AS (
			SELECT id FROM notes WHERE object->>'$.type' = 'Question' AND id LIKE $1 AND deleted = 0 AND object->>'$.closed' IS NULL
		),
		votes_per_voter AS (
			SELECT polls.id AS poll, votes.object->>'$.name' AS option, votes.author AS voter
			FROM polls
			LEFT JOIN notes votes ON votes.object->>'$.inReplyTo' = polls.id AND votes.deleted = 0 AND votes.object->>'$.name' IS NOT NULL
			GROUP BY poll, option, voter
		),
		voter_counts AS (
			SELECT polls.id AS poll, COUNT(DISTINCT voters.cid) AS total
			FROM polls
			JOIN notes votes ON votes.object->>'$.inReplyTo' = polls.id AND votes.deleted = 0 AND votes.object->>'$.name' IS NOT NULL
			JOIN persons voters ON voters.id = votes.author
			GROUP BY poll
		)
		SELECT vpv.poll, vpv.option, COUNT(vpv.voter), COALESCE(vc.total, 0)
		FROM votes_per_voter vpv
		LEFT JOIN voter_counts vc ON vpv.poll = vc.poll
		GROUP BY vpv.poll, vpv.option
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
