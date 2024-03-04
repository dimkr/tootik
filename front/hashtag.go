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

func (h *Handler) hashtag(w text.Writer, r *request, args ...string) {
	tag := args[1]

	h.showFeedPage(
		w,
		r,
		"Posts Tagged #"+tag,
		func(offset int) (*sql.Rows, error) {
			return r.Query(
				`select notes.object, persons.actor, null, notes.inserted from notes join hashtags on notes.id = hashtags.note left join (select object->>'$.inReplyTo' as id, count(*) as count from notes where inserted >= unixepoch() - 7*24*60*60 group by object->>'$.inReplyTo') replies on notes.id = replies.id left join persons on notes.author = persons.id where notes.public = 1 and hashtags.hashtag = $1 order by replies.count desc, notes.inserted/(24*60*60) desc, notes.inserted desc limit $2 offset $3`,
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
