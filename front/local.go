/*
Copyright 2023 - 2025 Dima Krasner

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

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) local(w text.Writer, r *Request, args ...string) {
	h.showFeedPage(
		w,
		r,
		"📡 Local Feed",
		func(offset int) (*sql.Rows, error) {
			return h.DB.QueryContext(
				r.Context,
				`
					select json(object), json(actor), json(sharer), inserted from
					(
						select notes.object, persons.actor, null as sharer, notes.inserted from persons
						join notes
						on notes.author = persons.id
						where notes.public = 1 and persons.ed25519privkey is not null
						union all
						select notes.object, persons.actor, sharers.actor as sharer, shares.inserted from persons sharers
						join shares
						on shares.by = sharers.id
						join notes
						on notes.id = shares.note
						join persons
						on persons.id = notes.author
						where notes.public = 1 and sharers.ed25519privkey is not null
					)
					order by inserted desc
					limit $1
					offset $2
				`,
				h.Config.PostsPerPage,
				offset,
			)
		},
		true,
	)
}
