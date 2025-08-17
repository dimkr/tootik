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

package front

import (
	"database/sql"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) follows(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select json(persons.actor), g.inserted/(24*60*60), follows.accepted from
		follows
		left join
		(
			select followed, max(inserted) as inserted from
			(
				select coalesce(sharer->>'$.id', author->>'$.id') as followed, inserted
				from feed
				where
					follower = $1 and
					inserted >= unixepoch() - 7*24*60*60
			)
			group by followed
		) g
		on
			g.followed = follows.followed
		join persons
		on
			persons.id = follows.followed
		where
			follows.follower = $1
		order by
			g.inserted/(24*60*60) desc,
			g.inserted desc,
			follows.inserted desc
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
	w.Title("⚡ Follows")

	i := 0
	var lastDay sql.NullInt64
	for rows.Next() {
		var actor ap.Actor
		var last sql.NullInt64
		var accepted sql.NullInt32
		if err := rows.Scan(&actor, &last, &accepted); err != nil {
			r.Log.Warn("Failed to list a followed user", "error", err)
			continue
		}

		if i > 0 && last != lastDay {
			w.Separator()
		}
		lastDay = last

		displayName := h.getActorDisplayName(&actor)

		if !accepted.Valid && last.Valid {
			w.Linkf("/users/outbox/"+trimScheme(actor.ID), "%s %s - pending approval", time.Unix(last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
		} else if !accepted.Valid {
			w.Linkf("/users/outbox/"+trimScheme(actor.ID), "%s - pending approval", displayName)
		} else if last.Valid && accepted.Int32 == 1 {
			w.Linkf("/users/outbox/"+trimScheme(actor.ID), "%s %s", time.Unix(last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
		} else if accepted.Int32 == 1 {
			w.Link("/users/outbox/"+trimScheme(actor.ID), displayName)
		} else if last.Valid {
			w.Linkf("/users/outbox/"+trimScheme(actor.ID), "%s %s - rejected", time.Unix(last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
		} else {
			w.Linkf("/users/outbox/"+trimScheme(actor.ID), "%s - rejected", displayName)
		}
		i++
	}

	if i == 0 {
		w.Text("No followed users.")
		return
	}
}
