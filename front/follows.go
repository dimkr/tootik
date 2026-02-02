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

package front

import (
	"database/sql"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/dbx"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) follows(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := dbx.QueryCollectRowsIgnore[struct {
		Actor    ap.Actor
		Last     sql.NullInt64
		Accepted sql.NullInt32
	}](
		r.Context,
		h.DB,
		func(err error) bool {
			r.Log.Warn("Failed to list a followed user", "error", err)
			return true
		},
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
			follows.inserted desc,
			follows.followed
		`,
		r.User.ID,
	)
	if err != nil {
		r.Log.Warn("Failed to list followed users", "error", err)
		w.Error()
		return
	}

	w.OK()
	w.Title("⚡ Follows")

	if len(rows) == 0 {
		w.Text("No followed users.")
	} else {
		var lastDay sql.NullInt64

		for i, row := range rows {
			if i > 0 && row.Last != lastDay {
				w.Separator()
			}
			lastDay = row.Last

			displayName := h.getActorDisplayName(&row.Actor)

			if !row.Accepted.Valid && row.Last.Valid {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s %s - pending approval", time.Unix(row.Last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
			} else if !row.Accepted.Valid {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s - pending approval", displayName)
			} else if row.Last.Valid && row.Accepted.Int32 == 1 {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s %s", time.Unix(row.Last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
			} else if row.Accepted.Int32 == 1 {
				w.Link("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), displayName)
			} else if row.Last.Valid {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s %s - rejected", time.Unix(row.Last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
			} else {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s - rejected", displayName)
			}

		}
	}
}
