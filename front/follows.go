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

package front

import (
	"database/sql"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"strings"
	"time"
)

func (h *Handler) follows(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select persons.actor, g.ninserted/(24*60*60) from
		(
			select followed, max(ninserted) as ninserted, max(finserted) as finserted from
			(
				select follows.followed, feed.inserted as ninserted, follows.inserted as finserted from
				follows
				join feed
				on
					feed.author->>'$.id' = follows.followed or
					feed.sharer->>'$.id' = follows.followed
				where
					follows.follower = $1 and
					feed.follower = $1 and
					feed.inserted >= unixepoch() - 7*24*60*60
				union all
				select follows.followed, null as ninserted, follows.inserted as finserted from
				follows
				where
					follows.follower = $1
			)
			group by followed
		) g
		join persons
		on
			persons.id = g.followed
		order by
			g.ninserted/(24*60*60) desc,
			g.ninserted desc,
			g.finserted desc
		`,
		r.User.ID,
	)
	if err != nil {
		r.Log.Warn("Failed to list followed users", "error", err)
		w.Error()
		return
	}

	defer rows.Close()

	w.OK()
	w.Title("⚡ Followed Users")

	i := 0
	var lastDay sql.NullInt64
	for rows.Next() {
		var actor ap.Actor
		var last sql.NullInt64
		if err := rows.Scan(&actor, &last); err != nil {
			r.Log.Warn("Failed to list a followed user", "error", err)
			continue
		}

		if i > 0 && last != lastDay {
			w.Separator()
		}
		lastDay = last

		displayName := h.getActorDisplayName(&actor)

		if last.Valid {
			w.Linkf("/users/outbox/"+strings.TrimPrefix(actor.ID, "https://"), "%s %s", time.Unix(last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
		} else {
			w.Link("/users/outbox/"+strings.TrimPrefix(actor.ID, "https://"), displayName)
		}

		i++
	}

	if i == 0 {
		w.Text("No followed users.")
		return
	}
}
