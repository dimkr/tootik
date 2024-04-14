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
	"database/sql"
	"time"

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) mentions(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	h.showFeedPage(
		w,
		r,
		"📞 Mentions",
		func(offset int) (*sql.Rows, error) {
			return r.Query(`
				select object, actor, sharer, inserted from
				(
					select notes.id, notes.object, persons.actor, notes.inserted, null as sharer from
					follows
					join
					persons
					on
						persons.id = follows.followed
					join
					notes
					on
						notes.author = follows.followed and
						(
							$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = $1)) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = $1))
						)
					where
						follows.follower = $1 and
						notes.inserted >= $2
					union all
					select notes.id, notes.object, authors.actor, shares.inserted, sharers.actor as sharer from
					follows
					join
					shares
					on
						shares.by = follows.followed
					join
					notes
					on
						notes.id = shares.note and
						(
							$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = $1)) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = $1))
						)
					join
					persons authors
					on
						authors.id = notes.author
					join
					persons sharers
					on
						sharers.id = follows.followed
					where
						follows.follower = $1 and
						shares.inserted >= $2
				)
				order by
					inserted desc
				limit $3
				offset $4`,
				r.User.ID,
				time.Now().Add(-time.Hour*24*7).Unix(),
				h.Config.PostsPerPage,
				offset,
			)
		},
		true,
	)
}
