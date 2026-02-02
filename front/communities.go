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
	rows, err := data.QueryCollectRowsIgnore[struct {
		ID, Username string
		Last         int64
	}](
		r.Context,
		h.DB,
		func(err error) bool {
			r.Log.Warn("Failed to scan community", "error", err)
			return true
		},
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

	w.OK()
	w.Title("🏕️ Communities")

	if len(rows) == 0 {
		w.Text("No communities.")
		return
	}

	for _, row := range rows {
		if r.User == nil {
			w.Linkf("/outbox/"+strings.TrimPrefix(row.ID, "https://"), "%s %s", time.Unix(row.Last, 0).Format(time.DateOnly), row.Username)
		} else {
			w.Linkf("/users/outbox/"+strings.TrimPrefix(row.ID, "https://"), "%s %s", time.Unix(row.Last, 0).Format(time.DateOnly), row.Username)
		}
	}
}
