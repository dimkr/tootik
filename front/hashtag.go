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

func (h *Handler) hashtag(w text.Writer, r *Request, args ...string) {
	tag := args[1]

	h.showFeedPage(
		w,
		r,
		"Posts Tagged #"+tag,
		func(offset int) (*sql.Rows, error) {
			return h.DB.QueryContext(
				r.Context,
				`select json(page.object), json(persons.actor), null, page.inserted, page.nreplies, page.nquotes, page.nshares, json(parent_authors.actor) from (
					select notes.id, notes.object, notes.author, notes.inserted, notes.nreplies, notes.nquotes, notes.nshares from
					notes
					join hashtags on
						notes.id = hashtags.note
					where
						notes.public = 1 and hashtags.hashtag = $1
					order by
						notes.nreplies desc, notes.inserted/(24*60*60) desc, notes.inserted desc
					limit $2
					offset $3
				) page
				join persons on
					page.author = persons.id
				left join notes parent_notes on
					parent_notes.id = page.object->>'$.inReplyTo'
				left join persons parent_authors on
					parent_authors.id = parent_notes.author
				order by
					page.nreplies desc, page.inserted/(24*60*60) desc, page.inserted desc`,
				tag,
				h.Config.PostsPerPage,
				offset,
			)
		},
		false,
	)

	w.Separator()

	if r.User == nil {
		w.Link("/search", "🔎 Posts by hashtag")
	} else {
		w.Link("/users/search", "🔎 Posts by hashtag")
	}
}
