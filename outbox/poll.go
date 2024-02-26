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
	"fmt"
	"github.com/dimkr/tootik/ap"
	"log/slog"
	"time"
)

type Poller struct {
	Domain string
	Log    *slog.Logger
	DB     *sql.DB
}

type pollResult struct {
	PollID, Option string
}

func (p *Poller) Run(ctx context.Context) error {
	rows, err := p.DB.QueryContext(ctx, `select poll, option, count(*) from (select polls.id as poll, votes.object->>'$.name' as option, votes.author as voter from notes polls join notes votes on votes.object->>'$.inReplyTo' = polls.id where polls.object->>'$.type' = 'Question' and polls.id like $1 and polls.object->>'$.closed' is null and votes.object->>'$.name' is not null group by poll, option, voter) group by poll, option`, fmt.Sprintf("https://%s/%%", p.Domain))
	if err != nil {
		return err
	}
	defer rows.Close()

	votes := map[pollResult]int64{}
	polls := map[string]*ap.Object{}

	for rows.Next() {
		var pollID, option string
		var count int64
		if err := rows.Scan(&pollID, &option, &count); err != nil {
			p.Log.Warn("Failed to scan poll result", "error", err)
			continue
		}

		if _, ok := polls[pollID]; ok {
			votes[pollResult{PollID: pollID, Option: option}] = count
			continue
		}

		var obj ap.Object
		if err := p.DB.QueryRowContext(ctx, "select object from notes where id = ?", pollID).Scan(&obj); err != nil {
			p.Log.Warn("Failed to fetch poll", "poll", pollID, "error", err)
			continue
		}

		polls[pollID] = &obj
		votes[pollResult{PollID: pollID, Option: option}] = count
	}
	rows.Close()

	now := ap.Time{Time: time.Now()}

	for _, poll := range polls {
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

		if poll.EndTime == nil || now.After(poll.EndTime.Time) {
			poll.Closed = &now
			changed = true
		}

		if !changed {
			continue
		}

		p.Log.Info("Updating poll results", "poll", poll.ID)

		if err := UpdateNote(ctx, p.Domain, p.DB, poll); err != nil {
			p.Log.Warn("Failed to update poll results", "poll", poll.ID, "error", err)
		}
	}

	return nil
}
