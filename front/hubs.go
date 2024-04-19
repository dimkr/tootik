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

func (h *Handler) hubs(w text.Writer, r *request, args ...string) {
	rows, err := r.Query(
		`
			select id, actor->>'preferredUsername' from persons
			where
				host = $1 and
				actor->>'$.type' = 'Group'
		`,
		h.Domain,
	)
	if err != nil {
		r.Log.Warn("Failed to list hubs", "error", err)
		w.Error()
		return
	}

	w.OK()

	w.Title("🏛️ Hubs")

	for rows.Next() {
		var id, username string
		if err := rows.Scan(&id, &username); err != nil {
			r.Log.Warn("Failed to scan hub", "error", err)
			continue
		}

		if r.User == nil {
			w.Link("/outbox/"+strings.TrimPrefix(id, "https://"), username)
		} else {
			w.Link("/users/outbox/"+strings.TrimPrefix(id, "https://"), username)
		}
	}

	rows.Close()
}
