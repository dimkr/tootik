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
	"fmt"
	"path/filepath"
	"time"

	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) dailyPosts(w text.Writer, r *request, day time.Time) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := time.Now()
	if day.After(now) {
		r.Log.Info("Date is in the future", "day", day)
		w.Redirect("/users/oops")
		return
	}

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "url", r.URL, "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.Log.Info("Viewing inbox", "offset", offset)

	rows, err := r.Query(`
		select gup.object, gup.actor, gup.g from
		(
			select u.id, u.object, u.author, u.cc0, u.to0, u.cc1, u.to1, u.cc2, u.to2, u.inserted, authors.actor, groups.actor as g from
			(
				select notes.id, notes.object, notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2, notes.inserted, notes.groupid from
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
						notes.groupid = followed.id
					)
				where
					follows.follower = $1 and
					notes.inserted >= $2 and
					notes.inserted < $2 + 60*60*24
				union
				select notes.id, notes.object, notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2, notes.inserted, notes.groupid from
				notes myposts
				join
				notes
				on
					notes.object->>'inReplyTo' = myposts.id
				where
					myposts.author = $1 and
					notes.author != $1 and
					notes.inserted >= $2 and
					notes.inserted < $2 + 60*60*24
			) u
			join
			persons authors
			on
			authors.id = u.author
			left join
			persons groups
			on
			groups.actor->>'type' = 'Group' and groups.id = u.groupid
		) gup
		left join (
			select author, round(count(*) / 24.0, 1) as avg from notes where inserted >= $2 and inserted < $2 + 60*60*24 group by author
		) stats
		on
			stats.author = gup.author
		left join (
			select notes.object, notes.author, follows.id as follow from notes left join follows on follows.followed = notes.author and follows.follower = $1 where notes.inserted >= unixepoch()-2*24*60*60
		) replies
		on
			replies.object->>'inReplyTo' = gup.id and replies.author != gup.author
		group by gup.id
		order by
			(case
				when gup.to0 = $1 and gup.to1 is null and gup.cc0 is null then 0
				when $1 in (gup.cc0, gup.to0, gup.cc1, gup.to1, gup.cc2, gup.to2) or (gup.to2 is not null and exists (select 1 from json_each(gup.object->'to') where value = $1)) or (gup.cc2 is not null and exists (select 1 from json_each(gup.object->'cc') where value = $1)) then 1
				else 2
			end),
			count(distinct replies.follow) desc,
			count(distinct replies.author) desc,
			stats.avg asc,
			gup.inserted / 3600 desc,
			gup.actor->>'type' = 'Person' desc,
			gup.inserted desc
		limit $3
		offset $4`, r.User.ID, day.Unix(), h.Config.PostsPerPage, offset)
	if err != nil {
		r.Log.Warn("Failed to fetch posts", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		noteString := ""
		var meta noteMetadata
		if err := rows.Scan(&noteString, &meta.Author, &meta.Group); err != nil {
			r.Log.Warn("Failed to scan post", "error", err)
			continue
		}

		notes.Store(noteString, meta)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Titlef("📻 Posts From %s (%d-%d)", day.Format(time.DateOnly), offset, offset+h.Config.PostsPerPage)
	} else {
		w.Titlef("📻 Posts From %s", day.Format(time.DateOnly))
	}

	if count == 0 {
		w.Text("No posts.")
	} else {
		r.PrintNotes(w, notes, true, true, false)
	}

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Separator()
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}
	if count == h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}
}

func (h *Handler) byDate(w text.Writer, r *request) {
	day, err := time.Parse(time.DateOnly, filepath.Base(r.URL.Path))
	if err != nil {
		r.Log.Info("Failed to parse date", "error", err)
		w.Status(40, "Invalid date")
		return
	}

	h.dailyPosts(w, r, day)
}

func (h *Handler) today(w text.Writer, r *request) {
	h.dailyPosts(w, r, time.Unix(time.Now().Unix()/86400*86400, 0))
}

func (h *Handler) yesterday(w text.Writer, r *request) {
	h.dailyPosts(w, r, time.Unix((time.Now().Unix()/86400-1)*86400, 0))
}
