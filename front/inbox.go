/*
Copyright 2023 Dima Krasner

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
	"fmt"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/text"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile("^/users$")] = withUserMenu(inbox)
}

func inbox(w text.Writer, r *request) {
	if r.User == nil {
		w.Status(61, "Peer certificate is required")
		return
	}

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.WithField("url", r.URL.String()).WithError(err).Info("Failed to parse query")
		w.Status(40, "Invalid query")
		return
	}

	r.Log.WithField("offset", offset).Info("Viewing inbox")

	now := time.Now()
	since := now.Add(-time.Hour * 24 * 2)

	rows, err := r.Query(`
		select notes.object, persons.actor from
		notes
		join (
			select follows.followed, persons.actor->>'followers' as followers, stats.avg, stats.last, persons.actor->>'type' as type from
			(
				select followed from follows where follower = $1
			) follows
			join
			persons
			on
				follows.followed = persons.id
			left join
			(
				select author, max(inserted) as last, count(*) / $2 as avg from notes where inserted >= $3 group by author
			) stats
			on
				stats.author = persons.id
		) follows
		on
			$1 in (notes.to0, notes.to1, notes.to2, notes.cc0, notes.cc1, notes.cc2) or
			follows.followers in (notes.to0, notes.to1, notes.to2, notes.cc0, notes.cc1, notes.cc2) or
			(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value in ($1, follows.followers))) or
			(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value in ($1, follows.followers))) or
			(
				(
					'https://www.w3.org/ns/activitystreams#Public' in (notes.to0, notes.to1, notes.to2, notes.cc0, notes.cc1, notes.cc2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = 'https://www.w3.org/ns/activitystreams#Public')) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = 'https://www.w3.org/ns/activitystreams#Public'))
				)
				and
				(
					follows.followed in (notes.author, notes.to0, notes.to1, notes.to2, notes.cc0, notes.cc1, notes.cc2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.followed)) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.followed))
				)
			)
		left join (
			select object->>'inReplyTo' as id, count(*) as count from notes where inserted >= $3 group by object->>'inReplyTo'
		) replies
		on
			replies.id = notes.id
		left join persons
		on
			persons.id = notes.author
		group by notes.id
		order by
			notes.inserted / 86400 desc,
			replies.count desc,
			follows.avg asc,
			follows.last asc,
			notes.inserted / 3600 desc,
			follows.type = 'Person' desc,
			notes.inserted desc
		limit $4
		offset $5`, r.User.ID, now.Sub(since)/time.Hour, since.Unix(), postsPerPage, offset)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to fetch posts")
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, sql.NullString]{}

	for rows.Next() {
		noteString := ""
		var actorString sql.NullString
		if err := rows.Scan(&noteString, &actorString); err != nil {
			r.Log.WithError(err).Warn("Failed to scan post")
			continue
		}

		notes.Store(noteString, actorString)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= postsPerPage || count == postsPerPage {
		w.Titlef("📻 My Radio (%d-%d)", offset, offset+postsPerPage)
	} else {
		w.Title("📻 My Radio")
	}

	printNotes(w, r, notes, true, true)

	if offset >= postsPerPage || count == postsPerPage {
		w.Separator()
	}

	if offset >= postsPerPage {
		w.Linkf(fmt.Sprintf("/users?%d", offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	}
	if count == postsPerPage {
		w.Linkf(fmt.Sprintf("/users?%d", offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	}
}
