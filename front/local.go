/*
Copyright 2023, 2024 Dima Krasner

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

func (h *Handler) local(w text.Writer, r *request, args ...string) {
	h.showFeedPage(
		w,
		r,
		"📡 This Planet",
		func(offset int) (*sql.Rows, error) {
			return r.Query(
				`
					select u.object, u.actor, u.by from
					(
						select notes.id, notes.author, notes.object, notes.inserted, persons.actor, null as by from persons
						join notes
						on notes.author = persons.id
						where notes.public = 1 and persons.host = $1
						union
						select notes.id, notes.author, notes.object, notes.inserted, persons.actor, sharers.actor as by from persons sharers
						join shares
						on shares.by = sharers.id
						join notes
						on notes.id = shares.note
						join persons
						on persons.id = notes.author
						where notes.public = 1 and notes.host != $1 and sharers.host = $1
					) u
					left join (
						select object->>'inReplyTo' as id, count(*) as count from notes
						where host = $1 and inserted > unixepoch()-60*60*24*7
						group by object->>'inReplyTo'
					) replies
					on
						replies.id = u.id
					left join (
						select author, max(inserted) as last, round(count(*)/7.0, 1) as avg from notes
						where host = $1 and inserted > unixepoch()-60*60*24*7
						group by author
					) stats
					on
						stats.author = u.author
					group by
						u.id
					order by
						u.inserted / 86400 desc,
						replies.count desc,
						stats.avg asc,
						stats.last asc,
						u.inserted desc
					limit $2
					offset $3
				`,
				h.Domain,
				h.Config.PostsPerPage,
				offset,
			)
		},
		true,
	)
}
