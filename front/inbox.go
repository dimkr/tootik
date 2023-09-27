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
	"github.com/dimkr/tootik/text"
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

	since := now.Add(-time.Hour * 24 * 2)

	rows, err := r.Query(`
		select notes.object, persons.actor, (case when follows.actor->>'type' = 'Group' then follows.actor else null end) from
		notes
		left join (
			select follows.followed, persons.actor, stats.avg, stats.last from
			(
				select followed from follows where follower = $1 and accepted = 1
			) follows
			join
			persons
			on
				follows.followed = persons.id
			left join
			(
				select author, max(inserted) / 86400 as last, count(*) / $2 as avg from notes where inserted >= $3 group by author
			) stats
			on
				stats.author = persons.id
		) follows
		on
			(
				notes.author = follows.followed and
				(
					notes.public = 1 or
					follows.actor->>'followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.actor->>'followers' or value = $1)) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.actor->>'followers' or value = $1))
				)
			)
			or
			(
				follows.actor->>'type' = 'Group' and
				notes.groupid = follows.followed and
				(
					(notes.public = 1 and notes.object->>'inReplyTo' is null) or
					$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = $1)) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = $1))
				)
			)
		left join (
			select id from notes where author = $1
		) myposts
		on
			myposts.id = notes.object->>'inReplyTo'
		left join (
			select notes.object->>'inReplyTo' as inreplyto, notes.author, follows.id as follow from notes left join follows on follows.follower = $1 and follows.followed = notes.author and follows.accepted = 1 where notes.inserted >= $3
		) replies
		on
			replies.inreplyto = notes.id and replies.author != notes.author
		left join persons
		on
			persons.id = notes.author
		where
			notes.inserted >= $4 and
			notes.inserted < $4 + 60*60*24 and
			(follows.followed is not null or (myposts.id is not null and notes.author != $1))
		group by notes.id
		order by
			notes.inserted / 86400 desc,
			(case
				when notes.to0 = $1 and notes.to1 is null and notes.cc0 is null then 0
				when $1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = $1)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = $1)) then 1
				else 2
			end),
			count(distinct replies.follow) desc,
			count(distinct replies.author) desc,
			follows.avg asc,
			follows.last asc,
			notes.inserted / 3600 desc,
			persons.actor->>'type' = 'Person' desc,
			notes.inserted desc
		limit $5
		offset $6`, r.User.ID, now.Sub(since)/time.Hour, since.Unix(), day.Unix(), postsPerPage, offset)
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
