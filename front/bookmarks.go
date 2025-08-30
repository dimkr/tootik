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
	"database/sql"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) bookmarks(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/oops")
		return
	}

	h.showFeedPage(
		w,
		r,
		"🔖 Bookmarks",
		func(offset int) (*sql.Rows, error) {
			return h.DB.QueryContext(
				r.Context,
				`select json(notes.object), json(persons.actor), null as sharer, notes.inserted from bookmarks
				join notes
				on
					notes.id = bookmarks.note
				join persons
				on
					persons.id = notes.author
				where
					bookmarks.by = $1 and 
					(
						notes.author = $1 or
						notes.public = 1 or
						exists (select 1 from json_each(notes.object->'$.to') where exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = $1 and follows.followed = notes.author and follows.accepted = 1 and (notes.author = value or persons.actor->>'$.followers' = value))) or
						exists (select 1 from json_each(notes.object->'$.cc') where exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = $1 and follows.followed = notes.author and follows.accepted = 1 and (notes.author = value or persons.actor->>'$.followers' = value))) or
						exists (select 1 from json_each(notes.object->'$.to') where value = $1) or
						exists (select 1 from json_each(notes.object->'$.cc') where value = $1)
					)
				order by bookmarks.inserted desc
				limit $2
				offset $3`,
				ap.Canonical(r.User.ID),
				h.Config.PostsPerPage,
				offset,
			)
		},
		false,
	)
}
