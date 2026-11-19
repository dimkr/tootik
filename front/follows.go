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

	rows, err := dbx.QueryCollectIgnore[struct {
		Actor    ap.Actor
		Last     sql.NullTime
		Accepted sql.NullBool
	}](
		r.Context,
		h.DBW,
		func(err error) bool {
			r.Log.Warn("Failed to list a followed user", "error", err)
			return true
		},
		`
		select persons.actor, date_trunc('day', g.inserted, 'UTC'), follows.accepted from
		follows
		left join
		(
			select followed, max(inserted) as inserted from
			(
				select coalesce(sharer, author) as followed, inserted
				from feed
				where
					follower = $1 and
					inserted >= now() - interval '7 days'
			) recent
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
			date_trunc('day', g.inserted, 'UTC') desc nulls last,
			g.inserted desc nulls last,
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
		var lastDay sql.NullTime

		for i, row := range rows {
			if i > 0 && (row.Last.Valid != lastDay.Valid || !row.Last.Time.Equal(lastDay.Time)) {
				w.Separator()
			}
			lastDay = row.Last

			displayName := h.getActorDisplayName(&row.Actor)

			if !row.Accepted.Valid && row.Last.Valid {
				w.Linkf("/users/outbox/"+idLink(row.Actor.ID), "%s %s - pending approval", row.Last.Time.UTC().Format(time.DateOnly), displayName)
			} else if !row.Accepted.Valid {
				w.Linkf("/users/outbox/"+idLink(row.Actor.ID), "%s - pending approval", displayName)
			} else if row.Last.Valid && row.Accepted.Bool {
				w.Linkf("/users/outbox/"+idLink(row.Actor.ID), "%s %s", row.Last.Time.UTC().Format(time.DateOnly), displayName)
			} else if row.Accepted.Bool {
				w.Link("/users/outbox/"+idLink(row.Actor.ID), displayName)
			} else if row.Last.Valid {
				w.Linkf("/users/outbox/"+idLink(row.Actor.ID), "%s %s - rejected", row.Last.Time.UTC().Format(time.DateOnly), displayName)
			} else {
				w.Linkf("/users/outbox/"+idLink(row.Actor.ID), "%s - rejected", displayName)
			}

		}
	}
}
