/*
Copyright 2024, 2025 Dima Krasner

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
	"time"

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) communities(w text.Writer, r *Request, args ...string) {
	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select u.id, u.username, max(u.inserted) from (
			select persons.id, persons.actor->>'preferredUsername' as username, shares.inserted from shares
			join persons
			on
				persons.id = shares.by
			where
				persons.ed25519privkey is not null and
				persons.actor->>'$.type' = 'Group'
			union all
			select persons.id, persons.actor->>'preferredUsername' as username, notes.inserted from notes
			join persons
			on
				persons.id = notes.author
			where
				persons.ed25519privkey is not null and
				persons.actor->>'$.type' = 'Group'
		) u
		group by
			u.id
		order by
			max(u.inserted) desc
		`,
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
		var last int64
		if err := rows.Scan(&id, &username, &last); err != nil {
			r.Log.Warn("Failed to scan community", "error", err)
			continue
		}

		if r.User == nil {
			w.Linkf("/outbox/"+trimScheme(id), "%s %s", time.Unix(last, 0).Format(time.DateOnly), username)
		} else {
			w.Linkf("/users/outbox/"+trimScheme(id), "%s %s", time.Unix(last, 0).Format(time.DateOnly), username)
		}

		empty = false
	}

	rows.Close()

	if empty {
		w.Text("No communities.")
	}
}
