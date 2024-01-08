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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
	"net/url"
)

func fts(w text.Writer, r *request) {
	if r.URL.RawQuery == "" {
		w.Status(10, "Query")
		return
	}

	query, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Info("Failed to decode query", "url", r.URL, "error", err)
		w.Status(40, "Bad input")
		return
	}

	var rows *sql.Rows
	if r.User == nil {
		rows, err = r.Query(
			`
				select notes.object, authors.actor, groups.actor from
				notesfts
				join notes on
					notes.id = notesfts.id
				join persons authors on
					authors.id = notes.author and coalesce(authors.actor->>'discoverable', 1)
				left join persons groups on
					groups.actor->>'type' = 'Group' and groups.id = notes.groupid
				where
					notes.public = 1 and
					notesfts.content match $1
				order by rank, notes.inserted desc
				limit $2
			`,
			query,
			30,
		)
	} else {
		rows, err = r.Query(
			`
				select u.object, authors.actor, groups.actor from
				(
					select notes.id, notes.object, notes.author, notes.inserted, notes.groupid, rank, 2 as aud from
					notesfts
					join notes on
						notes.id = notesfts.id
					where
						notes.public = 1 and
						notesfts.content match $1
					union
					select notes.id, notes.object, notes.author, notes.inserted, notes.groupid, rank, 1 as aud from
					follows
					join
					persons
					on
						persons.id = follows.followed
					join
					notes on
						(
							persons.actor->>'followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = persons.actor->>'followers')) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = persons.actor->>'followers'))
						)
					join
					notesfts on
						notesfts.id = notes.id
					where
						follows.follower = $2 and
						notesfts.content match $1
					union
					select notes.id, notes.object, notes.author, notes.inserted, notes.groupid, rank, 0 as aud from
					notesfts
					join notes on
						notes.id = notesfts.id
					where
						notesfts.content match $1 and
						(
							$2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = $2)) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = $2))
						)
				) u
				join persons authors on
					authors.id = u.author and coalesce(authors.actor->>'discoverable', 1)
				left join persons groups on
					groups.actor->>'type' = 'Group' and groups.id = u.groupid
				group by
					u.id
				order by
					u.rank,
					min(u.aud),
					u.inserted desc
				limit $3
			`,
			query,
			r.User.ID,
			30,
		)
	}
	if err != nil {
		r.Log.Warn("Failed to search for posts", "query", query, "error", err)
		w.Error()
		return
	}

	notes := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		noteString := ""
		var meta noteMetadata
		if err := rows.Scan(&noteString, &meta.Author, &meta.Group); err != nil {
			r.Log.Warn("Failed to scan search result", "error", err)
			continue
		}
		notes.Store(noteString, meta)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	w.Titlef("🔎 Search Results for '%s'", query)

	if count == 0 {
		w.Text("No results.")
	} else {
		r.PrintNotes(w, notes, true, true, false)
	}
}
