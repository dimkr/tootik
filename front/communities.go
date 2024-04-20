/*
Copyright 2024 Dima Krasner

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
	"strings"

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) communities(w text.Writer, r *request, args ...string) {
	rows, err := r.Query(
		`
			select persons.id, persons.actor->>'preferredUsername', count(follows.followed) as followers from persons
			left join follows
			on
				follows.followed = persons.id
			where
				persons.host = $1 and
				persons.actor->>'$.type' = 'Group'
			group by
				persons.id
			order by
				followers
		`,
		h.Domain,
	)
	if err != nil {
		r.Log.Error("Failed to list communities", "error", err)
		w.Error()
		return
	}

	w.OK()

	w.Title("🏕️ Communities")

	empty := true

	for rows.Next() {
		var id, username string
		var followers int64
		if err := rows.Scan(&id, &username, &followers); err != nil {
			r.Log.Warn("Failed to scan community", "error", err)
			continue
		}

		if r.User == nil {
			w.Linkf("/outbox/"+strings.TrimPrefix(id, "https://"), "%s (%d)", username, followers)
		} else {
			w.Linkf("/users/outbox/"+strings.TrimPrefix(id, "https://"), "%s (%d)", username, followers)
		}

		empty = false
	}

	rows.Close()

	if empty {
		w.Text("No communities.")
	}
}
