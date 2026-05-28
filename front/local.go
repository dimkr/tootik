/*
Copyright 2023 - 2026 Dima Krasner

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
					select json(notes.object), json(authors.actor), json(sharers.actor), page.inserted, notes.nreplies, notes.nquotes, notes.nshares, json(parent_authors.actor) from (
						select id, author, sharer, inserted from
						(
							select notes.id, notes.author, null as sharer, notes.inserted from persons
							join notes
							on notes.author = persons.id
							where notes.public = 1 and persons.host = $1
							union all
							select notes.id, notes.author, sharers.id as sharer, shares.inserted from persons sharers
							join shares
							on shares.by = sharers.id
							join notes
							on notes.id = shares.note
							where notes.public = 1 and sharers.host = $1
						)
						order by inserted desc
						limit $2
						offset $3
					) page
					join notes on
						notes.id = page.id
					join persons authors on
						authors.id = page.author
					left join notes parent_notes on
						parent_notes.id = notes.object->>'$.inReplyTo'
					left join persons parent_authors on
						parent_authors.id = parent_notes.author
					left join persons sharers on
						sharers.id = page.sharer
					order by
						page.inserted desc
				`,
				h.Domain,
				h.Config.PostsPerPage,
				offset,
			)
		},
		true,
	)
}
