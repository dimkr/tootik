/*
Copyright 2024 - 2026 Dima Krasner

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
	"time"

	"github.com/dimkr/tootik/data"
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
				persons.host = $1 and
				persons.actor->>'$.type' = 'Group'
			union all
			select persons.id, persons.actor->>'preferredUsername' as username, notes.inserted from notes
			join persons
			on
				persons.id = notes.author
			where
				persons.host = $1 and
				persons.actor->>'$.type' = 'Group'
		) u
		group by
			u.id
		order by
			max(u.inserted) desc
		`,
		h.Domain,
	)
	if err != nil {
		r.Log.Error("Failed to list communities", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	w.OK()

	w.Title("🏕️ Communities")

	empty := true

	if err := data.ScanRows(
		rows,
		func(row struct {
			ID, Username string
			Last         int64
		}) bool {
			if r.User == nil {
				w.Linkf("/outbox/"+strings.TrimPrefix(row.ID, "https://"), "%s %s", time.Unix(row.Last, 0).Format(time.DateOnly), row.Username)
			} else {
				w.Linkf("/users/outbox/"+strings.TrimPrefix(row.ID, "https://"), "%s %s", time.Unix(row.Last, 0).Format(time.DateOnly), row.Username)
			}

			empty = false
			return true
		},
		func(err error) bool {
			r.Log.Warn("Failed to scan community", "error", err)
			return true
		},
	); err != nil {
		r.Log.Error("Failed to list communities", "error", err)
		return
	}

	if empty {
		w.Text("No communities.")
	}
}
