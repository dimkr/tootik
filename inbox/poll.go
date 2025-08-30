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

package inbox

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	"github.com/dimkr/tootik/ap"
)

type Poller struct {
	*Queue
}

type pollResult struct {
	PollID, Option string
}

func (p *Poller) Run(ctx context.Context) error {
	rows, err := p.DB.QueryContext(ctx, `select poll, option, count(*) from (select polls.id as poll, votes.object->>'$.name' as option, votes.author as voter from notes polls left join notes votes on votes.parent = polls.id where polls.object->>'$.type' = 'Question' and polls.id like $1 and polls.object->>'$.closed' is null and (votes.object->>'$.name' is not null or votes.id is null) group by poll, option, voter) group by poll, option`, fmt.Sprintf("https://%s/%%", p.Domain))
	if err != nil {
		return err
	}
	defer rows.Close()

	votes := map[pollResult]int64{}
	polls := map[string]*ap.Object{}
	authors := map[string]*ap.Actor{}

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
		if err := p.DB.QueryRowContext(ctx, "select json(notes.object), json(persons.actor) from notes join persons on persons.id = notes.author where notes.id = ?", pollID).Scan(&obj, &author); err != nil {
			slog.Warn("Failed to fetch poll", "poll", pollID, "error", err)
			continue
		}

		polls[pollID] = &obj
		if option.Valid {
			votes[pollResult{PollID: pollID, Option: option.String}] = count
		}

		authors[pollID] = &author
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

		slog.Info("Updating poll results", "poll", poll.ID)

		if err := p.Queue.UpdateNote(ctx, p.DB, authors[pollID], poll); err != nil {
			slog.Warn("Failed to update poll results", "poll", poll.ID, "error", err)
		}
	}

	return nil
}
