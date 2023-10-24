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
	"fmt"
	"path/filepath"
	"time"

	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
)

func dailyPosts(w text.Writer, r *request, day time.Time) {
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
		select u.object, u.actor, (case when u.actor->>'type' = 'Group' then u.actor else null end) from
		(
			select notes.id, notes.object, notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2, notes.inserted, persons.actor from
			follows
			join
			persons
			on
				persons.id = follows.followed
			join
			notes
			on
				(
					notes.author = follows.followed and
					(
						notes.public = 1 or
						persons.actor->>'followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
						$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
						(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = persons.actor->>'followers' or value = $1)) or
						(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = persons.actor->>'followers' or value = $1))
					)
				)
				or
				(
					persons.actor->>'type' = 'Group' and
					notes.groupid = follows.followed and
					(
						(notes.public = 1 and notes.object->>'inReplyTo' is null) or
						$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
						(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = $1)) or
						(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = $1))
					)
				)
			where
				follows.follower = $1 and
				notes.inserted >= $2 and
				notes.inserted < $2 + 60*60*24
			union
			select notes.id, notes.object, notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2, notes.inserted, persons.actor from
			notes myposts
			join
			notes
			on
				notes.object->>'inReplyTo' = myposts.id
			join
			persons
			on
				persons.id = notes.author
			where
				myposts.author = $1 and
				notes.author != $1 and
				notes.inserted >= $2 and
				notes.inserted < $2 + 60*60*24
		) u
		left join (
			select author, max(inserted) / 86400 as last, count(*) / 48 as avg from notes where inserted >= unixepoch()-2*24*60*60 group by author
		) stats
		on
			stats.author = u.author
		left join (
			select notes.object, notes.author, follows.id as follow from notes left join follows on follows.followed = notes.author and follows.follower = $1 where notes.inserted >= unixepoch()-2*24*60*60
		) replies
		on
			replies.object->>'inReplyTo' = u.id and replies.author != u.author
		group by u.id
		order by
			(case
				when u.to0 = $1 and u.to1 is null and u.cc0 is null then 0
				when $1 in (u.cc0, u.to0, u.cc1, u.to1, u.cc2, u.to2) or (u.to2 is not null and exists (select 1 from json_each(u.object->'to') where value = $1)) or (u.cc2 is not null and exists (select 1 from json_each(u.object->'cc') where value = $1)) then 1
				else 2
			end),
			count(distinct replies.follow) desc,
			count(distinct replies.author) desc,
			stats.avg asc,
			stats.last asc,
			u.inserted / 3600 desc,
			u.actor->>'type' = 'Person' desc,
			u.inserted desc
		limit $3
		offset $4`, r.User.ID, day.Unix(), postsPerPage, offset)
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

	if offset >= postsPerPage || count == postsPerPage {
		w.Titlef("📻 Posts From %s (%d-%d)", day.Format(time.DateOnly), offset, offset+postsPerPage)
	} else {
		w.Titlef("📻 Posts From %s", day.Format(time.DateOnly))
	}

	if count == 0 {
		w.Text("No posts.")
	} else {
		r.PrintNotes(w, notes, true, true)
	}

	if offset >= postsPerPage || count == postsPerPage {
		w.Separator()
	}

	if offset >= postsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	}
	if count == postsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	}
}

func byDate(w text.Writer, r *request) {
	day, err := time.Parse(time.DateOnly, filepath.Base(r.URL.Path))
	if err != nil {
		r.Log.Info("Failed to parse date", "error", err)
		w.Status(40, "Invalid date")
		return
	}

	dailyPosts(w, r, day)
}

func today(w text.Writer, r *request) {
	dailyPosts(w, r, time.Unix(time.Now().Unix()/86400*86400, 0))
}

func yesterday(w text.Writer, r *request) {
	dailyPosts(w, r, time.Unix((time.Now().Unix()/86400-1)*86400, 0))
}
