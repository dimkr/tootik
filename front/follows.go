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

type followedUserActivity struct {
	Actor ap.Actor
	Last  sql.NullInt64
}

func (h *Handler) follows(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := r.Query(`
		select u.actor, max(u.ninserted)/(24*60*60) from
		(
			select persons.actor, notes.inserted as ninserted, follows.inserted as finserted from
			follows
			join persons
			on
				persons.id = follows.followed
			join notes
			on
				notes.object->>'audience' = follows.followed
			where
				follows.follower = $1 and
				persons.actor->>'type' = 'Group' and
				notes.object->'inReplyTo' is null and
				notes.inserted >= unixepoch() - 7*24*60*60
			union
			select persons.actor, notes.inserted as ninserted, follows.inserted as finserted from
			follows
			join persons
			on
				persons.id = follows.followed
			join notes
			on
				notes.author = follows.followed
			where
				follows.follower = $1 and
				persons.actor->>'type' != 'Group' and
				notes.inserted >= unixepoch() - 7*24*60*60
			union
			select persons.actor, shares.inserted as ninserted, follows.inserted as finserted from
			follows
			join shares
			on
				shares.by = follows.followed
			join persons
			on
				persons.id = follows.followed
			join notes
			on
				notes.id = shares.note
			where
				shares.inserted >= unixepoch() - 7*24*60*60 and
				notes.public = 1 and
				follows.follower = $1
			union
			select persons.actor, null as ninserted, follows.inserted as finserted from
			follows
			join persons
			on
				persons.id = follows.followed
			where
				follows.follower = $1
		) u
		group by
			u.actor
		order by
			max(u.ninserted)/(24*60*60) desc,
			max(u.ninserted) desc,
			u.finserted desc
		`,
		r.User.ID,
	)
	if err != nil {
		r.Log.Warn("Failed to list followed users", "error", err)
		w.Error()
		return
	}

	var followedUsers []followedUserActivity

	for rows.Next() {
		var row followedUserActivity
		if err := rows.Scan(&row.Actor, &row.Last); err != nil {
			r.Log.Warn("Failed to list a followed user", "error", err)
			continue
		}

		followedUsers = append(followedUsers, row)
	}
	rows.Close()

	w.OK()
	w.Title("⚡ Followed Users")

	if len(followedUsers) == 0 {
		w.Text("No followed users.")
		return
	}

	var lastDay sql.NullInt64
	for i, row := range followedUsers {
		if i > 0 && row.Last != lastDay {
			w.Separator()
		}
		lastDay = row.Last

		displayName := h.getActorDisplayName(&row.Actor, r.Log)

		if row.Last.Valid {
			w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s %s", time.Unix(row.Last.Int64*(60*60*24), 0).Format(time.DateOnly), displayName)
		} else {
			w.Link("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), displayName)
		}
	}
}
