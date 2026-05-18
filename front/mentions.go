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
	"database/sql"

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) mentions(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	h.showFeedPage(
		w,
		r,
		"📞 Mentions",
		func(offset int) (*sql.Rows, error) {
			return h.DB.QueryContext(
				r.Context,
				`select json(page.note), json(page.author), json(page.sharer), page.inserted, notes.replies_count, notes.quotes_count, notes.shares_count, parent_authors.actor->>'$.preferredUsername' from (
					select note, author, sharer, inserted from feed
					where
						follower = $1 and
						(
							exists (select 1 from json_each(note->'$.to') where value = $1) or
							exists (select 1 from json_each(note->'$.cc') where value = $1)
						)
					order by
						feed.inserted desc
					limit $2
					offset $3
				) page
				join notes on
					notes.id = page.note->>'$.id'
				left join notes parent_notes on
					parent_notes.id = notes.object->>'$.inReplyTo'
				left join persons parent_authors on
					parent_authors.id = parent_notes.author
				order by page.inserted desc
				`,
				r.User.ID,
				h.Config.PostsPerPage,
				offset,
			)
		},
		true,
	)
}
