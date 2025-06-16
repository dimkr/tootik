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
	"fmt"
	"net/url"
	"regexp"
	"strconv"

	"github.com/dimkr/tootik/front/text"
)

var skipRegex = regexp.MustCompile(` skip (\d+)$`)

func (h *Handler) fts(w text.Writer, r *Request, args ...string) {
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

	var offset int
	if loc := skipRegex.FindStringSubmatchIndex(query); loc != nil {
		offset64, err := strconv.ParseInt(query[loc[2]:loc[3]], 10, 32)
		if err != nil {
			r.Log.Info("Failed to parse offset", "query", query, "error", err)
			w.Status(40, "Invalid offset")
			return
		}

		offset = int(offset64)
		query = query[:loc[0]]
	}

	var rows *sql.Rows
	if r.User == nil {
		rows, err = h.DB.QueryContext(
			r.Context,
			`
				select notes.object, authors.actor, groups.actor, notes.inserted from
				notesfts
				join notes on
					notes.id = notesfts.id
				join persons authors on
					authors.id = notes.author and coalesce(authors.actor->>'$.discoverable', 1)
				left join persons groups on
					groups.actor->>'$.type' = 'Group' and exists (select 1 from shares where shares.by = groups.id and shares.note = notes.id)
				where
					notes.public = 1 and
					notesfts.content match $1
				order by rank desc
				limit $2
				offset $3
			`,
			query,
			h.Config.PostsPerPage,
			offset,
		)
	} else {
		rows, err = h.DB.QueryContext(
			r.Context,
			`
				select u.object, authors.actor, groups.actor, u.inserted from
				(
					select notes.id, notes.object, notes.author, notes.inserted, rank, 2 as aud from
					notesfts
					join notes on
						notes.id = notesfts.id
					where
						notes.public = 1 and
						notesfts.content match $1
					union all
					select notes.id, notes.object, notes.author, notes.inserted, rank, 1 as aud from
					follows
					join
					persons
					on
						persons.id = follows.followed
					join
					notes on
						(
							persons.actor->>'$.followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = persons.actor->>'$.followers')) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = persons.actor->>'$.followers')) or
							(persons.actor->>'$.type' = 'Group' and exists (select 1 from shares where shares.by = persons.id and shares.note = notes.id))
						)
					join
					notesfts on
						notesfts.id = notes.id
					where
						follows.follower = $2 and
						follows.accepted = 1 and
						notesfts.content match $1
					union all
					select notes.id, notes.object, notes.author, notes.inserted, rank, 0 as aud from
					notesfts
					join notes on
						notes.id = notesfts.id
					where
						notesfts.content match $1 and
						(
							$2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = $2)) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = $2))
						)
				) u
				join persons authors on
					authors.id = u.author and coalesce(authors.actor->>'$.discoverable', 1)
				left join persons groups on
					groups.actor->>'$.type' = 'Group' and exists (select 1 from shares where shares.by = groups.id and shares.note = u.id)
				group by
					u.id
				order by
					round(u.rank, 1),
					min(u.aud),
					u.rank
				limit $3
				offset $4
			`,
			query,
			r.User.ID,
			h.Config.PostsPerPage,
			offset,
		)
	}
	if err != nil {
		r.Log.Warn("Failed to search for posts", "query", query, "error", err)
		w.Error()
		return
	}

	w.OK()

	if offset > 0 {
		w.Titlef("🔎 Search Results for '%s' (%d-%d)", query, offset, offset+h.Config.PostsPerPage)
	} else {
		w.Titlef("🔎 Search Results for '%s'", query)
	}

	count := h.PrintNotes(w, r, rows, true, false, "No results.")
	rows.Close()

	if offset > 0 || count == h.Config.PostsPerPage {
		w.Empty()
		w.Subtitle("Navigation")
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%s", r.URL.Path, url.PathEscape(fmt.Sprintf("%s skip %d", query, offset-h.Config.PostsPerPage))), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%s", r.URL.Path, url.PathEscape(fmt.Sprintf("%s skip %d", query, offset+h.Config.PostsPerPage))), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}
}
