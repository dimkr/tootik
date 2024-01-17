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
	"encoding/json"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"strings"
	"time"
)

type followedUserActivity struct {
	Actor ap.Actor
	Last  sql.NullInt64
	Count sql.NullInt64
}

func (h *Handler) follows(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := r.Query(`with followed as (select followed as id, inserted from follows where follower = ?) select persons.actor, max(notes.inserted) as last, count(distinct notes.id) as count from followed join (select id, actor from persons) persons on persons.id = followed.id left join (select * from notes where inserted >= unixepoch() - 7*24*60*60) notes on (persons.actor->>'type' != 'Group' and notes.author = followed.id) or (persons.actor->>'type' = 'Group' and notes.object->'inReplyTo' is null and notes.groupid = followed.id) group by followed.id order by last desc, followed.inserted desc`, r.User.ID)
	if err != nil {
		r.Log.Warn("Failed to list followed users", "error", err)
		w.Error()
		return
	}

	var active []followedUserActivity
	var inactive []followedUserActivity

	for rows.Next() {
		var row followedUserActivity
		var actorString string
		if err := rows.Scan(&actorString, &row.Last, &row.Count); err != nil {
			r.Log.Warn("Failed to list a followed user", "error", err)
			continue
		}

		if err := json.Unmarshal([]byte(actorString), &row.Actor); err != nil {
			r.Log.Warn("Failed to unmarshal a followed user", "error", err)
			continue
		}

		if row.Last.Valid {
			active = append(active, row)
		} else {
			inactive = append(inactive, row)
		}
	}
	rows.Close()

	w.OK()
	w.Title("⚡ Followed Users")

	if len(active) == 0 && len(inactive) == 0 {
		w.Text("No followed users.")
		return
	}

	if len(active) > 0 {
		w.Text("Followed users who posted in the last week:")
		w.Empty()

		for _, row := range active {
			displayName := h.getActorDisplayName(&row.Actor, r.Log)

			if row.Count.Valid && row.Count.Int64 > 1 {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s %s: %d posts", time.Unix(row.Last.Int64, 0).Format(time.DateOnly), displayName, row.Count.Int64)
			} else if row.Count.Valid {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), "%s %s: 1 post", time.Unix(row.Last.Int64, 0).Format(time.DateOnly), displayName)
			} else {
				w.Link("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), displayName)
			}
		}
	}

	if len(inactive) > 0 {
		if len(active) > 0 {
			w.Empty()
		}
		w.Text("Followed users who haven't posted in the last week:")
		w.Empty()

		for _, row := range inactive {
			w.Link("/users/outbox/"+strings.TrimPrefix(row.Actor.ID, "https://"), h.getActorDisplayName(&row.Actor, r.Log))
		}
	}
}
