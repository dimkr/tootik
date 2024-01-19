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
	"time"

	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
)

func users(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Status(61, "Peer certificate is required")
		return
	}

	rows, err := r.Query(
		`
			select day*86400, count(*) from
			(
				select id, max(day) from
				(
					select notes.id, notes.inserted/86400 as day from
					notes
					join
					(
						select follows.followed, persons.actor->>'followers' as followers, persons.actor->>'type' as type from
						follows
						join
						persons
						on
							follows.followed = persons.id
					) follows
					on
						(
							(
								follows.type != 'Group' and notes.author = follows.followed and
								(
									notes.public = 1 or follows.followers in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
									$1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
									(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.followers or value = $1)) or
									(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.followers or value = $1))
								)
							) or
							(follows.type = 'Group' and follows.followed = notes.groupid)
						)
					where
						follows.follower = $1 and
						notes.inserted > unixepoch() - 60*60*24*7
					union
					select notes.id, shares.inserted/86400 as day from
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
						notes.id = shares.note
					where
						follows.follower = $1 and
						shares.inserted > unixepoch() - 60*60*24*7
					union
					select notes.id, notes.inserted/86400 as day, notes.object from
					notes
					join
					(
						select id from notes where author = $1
					) myposts
					on
						myposts.id = notes.object->>'inReplyTo'
					where
						notes.author != $1 and
						notes.inserted > unixepoch() - 60*60*24*7
				)
				group by
					id
			)
			group by
				day
			order by
				day desc
		`,
		r.User.ID,
	)
	if err != nil {
		r.Log.Warn("Failed to count posts", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	days := data.OrderedMap[int64, int64]{}

	for rows.Next() {
		var t, posts int64
		if err := rows.Scan(&t, &posts); err != nil {
			r.Log.Warn("Failed to scan row", "error", err)
			continue
		}

		days.Store(t, posts)
	}
	rows.Close()

	w.OK()

	w.Titlef("📻 My Radio")

	if len(days) == 0 {
		w.Text("Nothing to see! Are you following anyone?")
		return
	}

	today := time.Now().Unix() / 86400 * 86400
	yesterday := today - 86400

	days.Range(func(t, posts int64) bool {
		u := time.Unix(t, 0)
		s := u.Format(time.DateOnly)
		if t == today && posts > 1 {
			w.Linkf("/users/inbox/today", "%s Today, %s: %d posts", s, u.Weekday().String(), posts)
		} else if t == today {
			w.Linkf("/users/inbox/today", "%s Today, %s: 1 post", s, u.Weekday().String())
		} else if t == yesterday && posts > 1 {
			w.Linkf("/users/inbox/yesterday", "%s Yesterday, %s: %d posts", s, u.Weekday().String(), posts)
		} else if t == yesterday {
			w.Linkf("/users/inbox/yesterday", "%s Yesterday, %s: post", s, u.Weekday().String())
		} else if posts > 1 {
			w.Linkf("/users/inbox/"+s, "%s %s: %d posts", s, u.Weekday().String(), posts)
		} else {
			w.Linkf("/users/inbox/"+s, "%s %s: 1 post", s, u.Weekday().String())
		}
		return true
	})

	w.Empty()
	w.Link("/users/firehose", "🚿 Firehose")
}
