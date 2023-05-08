/*
Copyright 2023 Dima Krasner

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
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/text"
	"regexp"
	"time"
)

type followedUserActivity struct {
	Actor ap.Actor
	Last  sql.NullInt64
	Count sql.NullInt64
}

func init() {
	handlers[regexp.MustCompile(`^/users/follows$`)] = withUserMenu(follows)
}

func follows(w text.Writer, r *request) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := r.Query(`with followed as (select followed as id, inserted from follows where follower = ?) select persons.actor, stats.last, stats.count from followed join (select id, actor from persons) persons on persons.id = followed.id left join (select author, max(inserted) as last, count(*) as count from notes where inserted >= unixepoch() - 7*24*60*60 and exists (select 1 from followed where notes.author = followed.id) group by author) stats on stats.author = followed.id order by stats.last desc, followed.inserted desc`, r.User.ID)
	if err != nil {
		r.Log.WithField("follower", r.User.ID).WithError(err).Warn("Failed to list followed users")
		w.Error()
		return
	}

	var active []followedUserActivity
	var inactive []followedUserActivity

	for rows.Next() {
		var row followedUserActivity
		var actorString string
		if err := rows.Scan(&actorString, &row.Last, &row.Count); err != nil {
			r.Log.WithField("follower", r.User.ID).WithError(err).Warn("Failed to list a followed user")
			continue
		}

		if err := json.Unmarshal([]byte(actorString), &row.Actor); err != nil {
			r.Log.WithField("follower", r.User.ID).WithError(err).Warn("Failed to unmarshal a followed user")
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
			displayName := getActorDisplayName(&row.Actor)

			if row.Count.Valid && row.Count.Int64 > 1 {
				w.Linkf(fmt.Sprintf("%s/outbox/%x", r.AuthPrefix, sha256.Sum256([]byte(row.Actor.ID))), "%s %s: %d posts", time.Unix(row.Last.Int64, 0).Format(time.DateOnly), displayName, row.Count.Int64)
			} else if row.Count.Valid {
				w.Linkf(fmt.Sprintf("/%s/outbox/%x", r.AuthPrefix, sha256.Sum256([]byte(row.Actor.ID))), "%s %s: 1 post", time.Unix(row.Last.Int64, 0).Format(time.DateOnly), displayName)
			} else {
				w.Link(fmt.Sprintf("/%s/outbox/%x", r.AuthPrefix, sha256.Sum256([]byte(row.Actor.ID))), displayName)
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
			w.Link(fmt.Sprintf("/%s/outbox/%x", r.AuthPrefix, sha256.Sum256([]byte(row.Actor.ID))), getActorDisplayName(&row.Actor))
		}
	}
}
