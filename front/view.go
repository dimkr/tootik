/*
Copyright 2023 - 2025 Dima Krasner

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
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) view(w text.Writer, r *Request, args ...string) {
	postID := "https://" + args[1]

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.Log.Info("Viewing post", "post", postID)

	var note ap.Object
	var author ap.Actor
	var group sql.Null[ap.Actor]

	if r.User == nil {
		err = h.DB.QueryRowContext(
			r.Context,
			`
			select json(notes.object), json(persons.actor), json(groups.actor) from notes
			join persons on persons.id = notes.author
			left join (select id, actor from persons where actor->>'$.type' = 'Group') groups on exists (select 1 from shares where shares.by = groups.id and shares.note = $1)
			where
				notes.id = $1 and
				notes.public = 1
			`,
			postID,
		).Scan(&note, &author, &group)
	} else {
		err = h.DB.QueryRowContext(
			r.Context,
			`
			select json(notes.object), json(persons.actor), json(groups.actor) from notes
			join persons on persons.id = notes.author
			left join (select id, actor from persons where actor->>'$.type' = 'Group') groups on exists (select 1 from shares where shares.by = groups.id and shares.note = $1)
			where
				notes.id = $1 and
				(
					notes.public = 1 or
					notes.author = $2 or
					$2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = $2)) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = $2)) or
					exists (
						select 1 from (
							select persons.id, persons.actor->>'$.followers' as followers, persons.actor->>'$.type' as type from persons
							join follows on follows.followed = persons.id
							where
								follows.follower = $2 and
								follows.accepted = 1
						) follows
						where
							follows.followers in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = follows.followers)) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = follows.followers)) or
							(follows.type = 'Group' and exists (select 1 from shares where shares.by = follows.id and shares.note = notes.id))
					)
				)
			`,
			postID,
			r.User.ID,
		).Scan(&note, &author, &group)
	}
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Info("Post was not found", "post", postID)
		w.Status(40, "Post not found")
		return
	} else if err != nil {
		r.Log.Info("Failed to find post", "post", postID, "error", err)
		w.Error()
		return
	}

	var replies *sql.Rows
	if r.User == nil {
		replies, err = h.DB.QueryContext(
			r.Context,
			`
			select json(replies.object), json(persons.actor), null as sharer, replies.inserted from notes join notes replies on replies.object->>'$.inReplyTo' = notes.id
			left join persons on persons.id = replies.author
			where
				notes.id = $1 and
				replies.public = 1
			order by replies.inserted desc limit $2 offset $3
			`,
			postID,
			h.Config.RepliesPerPage,
			offset,
		)
	} else {
		replies, err = h.DB.QueryContext(
			r.Context,
			`
			select json(replies.object), json(persons.actor), null as sharer, replies.inserted from
			notes join notes replies on replies.object->>'$.inReplyTo' = notes.id
			left join persons on persons.id = replies.author
			where
				notes.id = $1 and
				(
					replies.public = 1 or
					replies.author = $2 or
					$2 in (replies.cc0, replies.to0, replies.cc1, replies.to1, replies.cc2, replies.to2) or
					(replies.to2 is not null and exists (select 1 from json_each(replies.object->'$.to') where value = $2)) or
					(replies.cc2 is not null and exists (select 1 from json_each(replies.object->'$.cc') where value = $2)) or
					exists (
						select 1 from persons
						join follows on follows.followed = persons.id
						where
							follows.follower = $2 and
							follows.accepted = 1 and
							(
								persons.actor->>'$.followers' in (replies.cc0, replies.to0, replies.cc1, replies.to1, replies.cc2, replies.to2) or
								(notes.to2 is not null and exists (select 1 from json_each(replies.object->'$.to') where value = persons.actor->>'$.followers')) or
								(notes.cc2 is not null and exists (select 1 from json_each(replies.object->'$.cc') where value = persons.actor->>'$.followers')) or
								(persons.actor->>'$.type' = 'Group' and exists (select 1 from shares where shares.by = persons.id and shares.note = replies.id))
							)
					)
				)
			order by replies.inserted desc limit $3 offset $4
			`,
			postID,
			r.User.ID,
			h.Config.RepliesPerPage,
			offset,
		)
	}
	if err != nil {
		r.Log.Info("Failed to fetch replies", "error", err)
		w.Error()
		return
	}

	w.OK()

	var threadHead sql.NullString

	if offset > 0 {
		w.Titlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+h.Config.RepliesPerPage)
	} else {
		if note.InReplyTo != "" {
			w.Titlef("💬 Reply by %s", author.PreferredUsername)

			w.Subtitle("Context")

			first := true
			if parents, err := h.DB.QueryContext(
				r.Context,
				`
				select json(note), json(author) from
				(
					with recursive thread(note, author, depth) as (
						select notes.object as note, persons.actor as author, 1 as depth
						from notes
						join persons on persons.id = notes.author
						where notes.id = ?
						union all
						select notes.object as note, persons.actor as author, t.depth + 1
						from thread t
						join notes on notes.id = t.note->>'$.inReplyTo'
						join persons on persons.id = notes.author
					)
					select * from thread order by depth limit ?
				)
				order by depth desc
				`,
				note.InReplyTo,
				h.Config.PostContextDepth,
			); err != nil {
				r.Log.Warn("Failed to fetch context", "error", err)
			} else {
				defer parents.Close()

				for parents.Next() {
					var parent ap.Object
					var parentAuthor ap.Actor
					if err := parents.Scan(&parent, &parentAuthor); err != nil {
						r.Log.Info("Failed to fetch context", "error", err)
						w.Error()
						return
					}

					if first && parent.InReplyTo != "" {
						w.Text("[…]")
						w.Empty()
					} else if !first {
						w.Empty()
					}

					if r.User == nil {
						w.Linkf("/view/"+strings.TrimPrefix(parent.ID, "https://"), "%s %s", parent.Published.Time.Format(time.DateOnly), parentAuthor.PreferredUsername)
					} else {
						w.Linkf("/users/view/"+strings.TrimPrefix(parent.ID, "https://"), "%s %s", parent.Published.Time.Format(time.DateOnly), parentAuthor.PreferredUsername)
					}

					contentLines, _ := h.getNoteContent(&parent, true)
					for _, line := range contentLines {
						w.Quote(line)
					}

					first = false

					if parent.InReplyTo == "" {
						threadHead.Valid = true
						threadHead.String = parent.ID
					}
				}

				if err := parents.Err(); err != nil {
					r.Log.Info("Failed to fetch context", "error", err)
					w.Error()
					return
				}
			}

			if first {
				w.Text("No context.")
			}

			w.Empty()
			w.Subtitle("Reply")
		} else if note.IsPublic() {
			w.Titlef("📣 Post by %s", author.PreferredUsername)
		} else {
			w.Titlef("🔔 Post by %s", author.PreferredUsername)
		}

		if group.Valid {
			h.PrintNote(w, r, &note, &author, &group.V, note.Published.Time, false, note.InReplyTo != "", false, false)
		} else {
			h.PrintNote(w, r, &note, &author, nil, note.Published.Time, false, note.InReplyTo != "", false, false)
		}

		if note.Type == ap.Question && offset == 0 {
			options := note.OneOf
			if len(options) == 0 {
				options = note.AnyOf
			}

			if len(options) > 0 {
				w.Empty()

				if note.VotersCount == 1 {
					w.Subtitle("📊 Results (one voter)")
				} else {
					w.Subtitlef("📊 Results (%d voters)", note.VotersCount)
				}

				labels := make([]string, 0, len(options))
				votes := make([]int64, 0, len(options))

				for _, option := range options {
					labels = append(labels, option.Name)
					votes = append(votes, option.Replies.TotalItems)
				}

				w.Raw("Results graph", graph.Bars(labels, votes))
			}
		}

		if offset > 0 {
			w.Empty()
			w.Subtitlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+h.Config.RepliesPerPage)
		} else {
			w.Empty()
			w.Subtitlef("💬 Replies to %s", author.PreferredUsername)
		}
	}

	count := h.PrintNotes(w, r, replies, false, false, "No replies.")
	replies.Close()

	if note.InReplyTo != "" && !threadHead.Valid {
		if err := h.DB.QueryRowContext(r.Context, `with recursive thread(id, parent, depth) as (select notes.id, notes.object->>'$.inReplyTo' as parent, 1 as depth from notes where id = ? union all select notes.id, notes.object->>'$.inReplyTo' as parent, t.depth + 1 from thread t join notes on notes.id = t.parent) select id from thread order by depth desc limit 1`, note.InReplyTo).Scan(&threadHead); err != nil && errors.Is(err, sql.ErrNoRows) {
			r.Log.Debug("First post in thread is missing")
		} else if err != nil {
			r.Log.Warn("Failed to fetch first post in thread", "error", err)
		}
	}

	var threadDepth int
	if err := h.DB.QueryRowContext(r.Context, `with recursive thread(id, depth) as (select notes.id, 0 as depth from notes where id = ? union all select notes.id, t.depth + 1 from thread t join notes on notes.object->>'$.inReplyTo' = t.id where t.depth <= 3) select max(thread.depth) from thread`, note.ID).Scan(&threadDepth); err != nil {
		r.Log.Warn("Failed to query thread depth", "error", err)
	}

	if (threadHead.Valid && threadHead.String != note.ID && threadHead.String != note.InReplyTo) || threadDepth > 2 || offset > h.Config.RepliesPerPage || offset >= h.Config.RepliesPerPage || count == h.Config.RepliesPerPage {
		w.Empty()
		w.Subtitle("Navigation")
	}

	if threadHead.Valid && threadHead.String != note.ID && threadHead.String != note.InReplyTo && r.User == nil {
		w.Link("/view/"+strings.TrimPrefix(threadHead.String, "https://"), "View first post in thread")
	} else if threadHead.Valid && threadHead.String != note.ID && threadHead.String != note.InReplyTo {
		w.Link("/users/view/"+strings.TrimPrefix(threadHead.String, "https://"), "View first post in thread")
	}

	if threadDepth > 2 && r.User == nil {
		w.Link("/thread/"+strings.TrimPrefix(postID, "https://"), "View thread")
	} else if threadDepth > 2 {
		w.Link("/users/thread/"+strings.TrimPrefix(postID, "https://"), "View thread")
	}

	if offset > h.Config.RepliesPerPage {
		w.Link(r.URL.Path, "First page")
	}

	if offset >= h.Config.RepliesPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.RepliesPerPage), "Previous page (%d-%d)", offset-h.Config.RepliesPerPage, offset)
	}

	if count == h.Config.RepliesPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.RepliesPerPage), "Next page (%d-%d)", offset+h.Config.RepliesPerPage, offset+2*h.Config.RepliesPerPage)
	}
}
