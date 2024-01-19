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

func (h *Handler) firehose(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	h.showFeedPage(
		w,
		r,
		"🚿 Firehose",
		func(offset int) (*sql.Rows, error) {
			return r.Query(`
				select gup.object, gup.actor, gup.g from
				(
					select u.id, u.object, u.inserted, authors.actor, groups.actor as g from
					(
						select notes.id, notes.object, notes.author, notes.inserted from
						follows
						join
						persons followed
						on
							followed.id = follows.followed
						join
						notes
						on
							(
								notes.author = follows.followed and
								(
									notes.public = 1 or
									followed.actor->>'followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
									$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
									(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = followed.actor->>'followers' or value = $1)) or
									(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = followed.actor->>'followers' or value = $1))
								)
							)
							or
							(
								followed.actor->>'type' = 'Group' and
								notes.object->>'audience' = followed.id
							)
						where
							follows.follower = $1 and
							notes.inserted >= $2
						union
						select notes.id, notes.object, notes.author, shares.inserted from
						follows
						join
						persons followed
						on
							followed.id = follows.followed
						join
						shares
						on
							shares.by = followed.id
						join
						notes
						on
							notes.id = shares.note and notes.object->>'audience' != shares.by
						where
							follows.follower = $1 and
							shares.inserted >= $2
						union
						select notes.id, notes.object, notes.author, notes.inserted from
						notes myposts
						join
						notes
						on
							notes.object->>'inReplyTo' = myposts.id
						where
							myposts.author = $1 and
							notes.author != $1 and
							notes.inserted >= $2
					) u
					join
					persons authors
					on
						authors.id = u.author
					left join
					persons groups
					on
						groups.actor->>'type' = 'Group' and groups.id = u.object->>'audience'
				) gup
				group by
					gup.id
				order by
					max(gup.inserted) desc
				limit $3
				offset $4`,
				r.User.ID,
				time.Now().Add(-time.Hour*24).Unix(),
				h.Config.PostsPerPage,
				offset,
			)
		},
		false,
	)
}
