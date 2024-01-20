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
	"time"

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

	r.Log.Info("Viewing inbox")

	h.showFeedPage(
		w,
		r,
		"📻 Posts From "+day.Format(time.DateOnly),
		func(offset int) (*sql.Rows, error) {
			return r.Query(`
				select gup.object, gup.actor, gup.sharer from
				(
					select u.id, u.object, u.author, u.cc0, u.to0, u.cc1, u.to1, u.cc2, u.to2, u.inserted, authors.actor, u.sharer from
					(
						select notes.id, notes.object, notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2, notes.inserted, null as sharer from
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
							notes.inserted >= $2 and
							notes.inserted < $2 + 60*60*24
						union
						select notes.id, notes.object, notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2, shares.inserted, followed.actor from
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
							notes.id = shares.note and (notes.object->>'audience' is null or notes.object->>'audience' != shares.by)
						where
							follows.follower = $1 and
							shares.inserted >= $2 and
							shares.inserted < $2 + 60*60*24 and
							notes.public = 1
						union
						select notes.id, notes.object, notes.author, notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2, notes.inserted, null as sharer from
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
				) gup
				left join (
					select author, round(count(*) / 24.0, 1) as avg from notes where inserted >= $2 and inserted < $2 + 60*60*24 group by author
				) stats
				on
					stats.author = gup.author
				left join (
					select notes.object->>'inReplyTo' as inReplyTo, notes.author, follows.id as follow from notes left join follows on follows.followed = notes.author and follows.follower = $1 where notes.inserted >= unixepoch()-2*24*60*60
				) replies
				on
					replies.inReplyTo = gup.id and replies.author != gup.author and replies.author != $1
				group by gup.id
				order by
					(case
						when gup.to0 = $1 and gup.to1 is null and gup.cc0 is null then 0
						when $1 in (gup.cc0, gup.to0, gup.cc1, gup.to1, gup.cc2, gup.to2) or (gup.to2 is not null and exists (select 1 from json_each(gup.object->'to') where value = $1)) or (gup.cc2 is not null and exists (select 1 from json_each(gup.object->'cc') where value = $1)) then 1
						else 2
					end),
					count(distinct replies.follow) desc,
					count(distinct gup.sharer) desc,
					count(distinct replies.author) desc,
					stats.avg asc,
					max(gup.inserted) / 3600 desc,
					gup.actor->>'type' = 'Person' desc,
					gup.inserted desc
				limit $3
				offset $4`,
				r.User.ID,
				day.Unix(),
				h.Config.PostsPerPage,
				offset,
			)
		},
		false,
	)
}

func (h *Handler) byDate(w text.Writer, r *request, args ...string) {
	day, err := time.Parse(time.DateOnly, args[1])
	if err != nil {
		r.Log.Info("Failed to parse date", "error", err)
		w.Status(40, "Invalid date")
		return
	}

	h.dailyPosts(w, r, day)
}

func (h *Handler) today(w text.Writer, r *request, args ...string) {
	h.dailyPosts(w, r, time.Unix(time.Now().Unix()/86400*86400, 0))
}

func (h *Handler) yesterday(w text.Writer, r *request, args ...string) {
	h.dailyPosts(w, r, time.Unix((time.Now().Unix()/86400-1)*86400, 0))
}
