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

func init() {
	handlers[regexp.MustCompile(`^/users/follows$`)] = withUserMenu(follows)
}

func follows(w text.Writer, r *request) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	since := time.Now().Add(-time.Hour * 24 * 7)

	rows, err := r.Query(`select persons.actor, stats.last, stats.count from (select followed, inserted from follows where follower = ?) follows join (select id, actor from persons) persons on persons.id = follows.followed left join (select author, max(inserted) as last, count(*) as count from notes where inserted >= ? group by author) stats on stats.author = follows.followed order by stats.last desc, follows.inserted desc`, r.User.ID, since.Unix())
	if err != nil {
		r.Log.WithField("follower", r.User.ID).WithError(err).Warn("Failed to list followed users")
		w.Error()
		return
	}

	w.OK()
	w.Title("⚡ Followed Users")

	sinceString := since.Format(time.DateOnly)

	for rows.Next() {
		var actorString string
		var lastOrNull sql.NullInt64
		var countOrNull sql.NullInt64
		if err := rows.Scan(&actorString, &lastOrNull, &countOrNull); err != nil {
			r.Log.WithField("follower", r.User.ID).WithError(err).Warn("Failed to list a followed user")
			continue
		}

		followed := ap.Actor{}
		if err := json.Unmarshal([]byte(actorString), &followed); err != nil {
			r.Log.WithField("follower", r.User.ID).WithError(err).Warn("Failed to unmarshal a followed user")
			continue
		}

		displayName := getActorDisplayName(&followed)

		if countOrNull.Valid {
			w.Linkf(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(followed.ID))), "%s ┃ %d since %s, last %s", displayName, countOrNull.Int64, sinceString, time.Unix(lastOrNull.Int64, 0).Format(time.DateOnly))
		} else {
			w.Link(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(followed.ID))), displayName)
		}
	}
}
