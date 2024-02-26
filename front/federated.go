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

func (h *Handler) federated(w text.Writer, r *request, args ...string) {
	h.showFeedPage(
		w,
		r,
		"✨️ FOMO From Outer Space",
		func(offset int) (*sql.Rows, error) {
			return r.Query(
				`select notes.object, persons.actor, groups.actor from notes join persons on notes.author = persons.id left join (select author, max(inserted) as last, count(*)/(60*60*24) as avg from notes where inserted > unixepoch()-60*60*24*7 group by author) stats on notes.author = stats.author left join (select id, actor from persons where actor->>'$.type' = 'Group') groups on groups.id = notes.object->>'$.audience' where notes.public = 1 group by notes.id order by notes.inserted / 3600 desc, stats.avg asc, stats.last asc, notes.inserted desc limit $1 offset $2`,
				h.Config.PostsPerPage,
				offset,
			)
		},
		false,
	)
}
