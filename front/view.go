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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
	"strings"
)

func (h *Handler) view(w text.Writer, r *request, args ...string) {
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
	var group ap.NullActor

	if r.User == nil {
		err = r.QueryRow(`select notes.object, persons.actor, groups.actor from notes join persons on persons.id = notes.author left join (select id, actor from persons where actor->>'type' = 'Group') groups on groups.id = notes.object->>'audience' where notes.id = ? and notes.public = 1`, postID).Scan(&note, &author, &group)
	} else {
		err = r.QueryRow(`select notes.object, persons.actor, groups.actor from notes join persons on persons.id = notes.author left join (select id, actor from persons where actor->>'type' = 'Group') groups on groups.id = notes.object->>'audience' where notes.id = $1 and (notes.public = 1 or notes.author = $2 or $2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = $2)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = $2)) or exists (select 1 from (select persons.id, persons.actor->>'followers' as followers, persons.actor->>'type' as type from persons join follows on follows.followed = persons.id where follows.accepted = 1 and follows.follower = $2) follows where follows.followers in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.followers)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.followers)) or (follows.id = notes.object->>'audience' and follows.type = 'Group')))`, postID, r.User.ID).Scan(&note, &author, &group)
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

	r.AddLogContext("post", note.ID)

	var rows *sql.Rows
	if r.User == nil {
		rows, err = r.Query(
			`select replies.object, persons.actor from notes join notes replies on replies.object->>'inReplyTo' = notes.id
			left join persons on persons.id = replies.author
			where notes.id = $1 and replies.public = 1
			order by replies.inserted desc limit $2 offset $3`,
			postID,
			h.Config.RepliesPerPage,
			offset,
		)
	} else {
		rows, err = r.Query(
			`select replies.object, persons.actor from
			notes join notes replies on replies.object->>'inReplyTo' = notes.id
			left join persons on persons.id = replies.author
			left join (select id from persons where actor->>'type' = 'Group') groups on groups.id = replies.object->>'audience'
			where notes.id = $1 and (replies.public = 1 or replies.author = $2 or $2 in (replies.cc0, replies.to0, replies.cc1, replies.to1, replies.cc2, replies.to2) or (replies.to2 is not null and exists (select 1 from json_each(replies.object->'to') where value = $2)) or (replies.cc2 is not null and exists (select 1 from json_each(replies.object->'cc') where value = $2)) or exists (select 1 from persons join follows on follows.followed = persons.id where follows.follower = $2 and follows.accepted = 1 and persons.actor->>'followers' in (replies.cc0, replies.to0, replies.cc1, replies.to1, replies.cc2, replies.to2) or (notes.to2 is not null and exists (select 1 from json_each(replies.object->'to') where value = persons.actor->>'followers')) or (notes.cc2 is not null and exists (select 1 from json_each(replies.object->'cc') where value = persons.actor->>'followers')) or (follows.followed = groups.id and persons.actor->>'type' = 'Group')))
			order by replies.inserted desc limit $3 offset $4`,
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
	defer rows.Close()

	replies := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		var meta noteMetadata
		if err := rows.Scan(&meta.Note, &meta.Author); err != nil {
			r.Log.Warn("Failed to scan reply", "error", err)
			continue
		}

		replies.Store(meta.Note.ID, meta)
	}
	rows.Close()

	count := len(replies)

	w.OK()

	if offset > 0 {
		w.Titlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+h.Config.RepliesPerPage)
	} else {
		if r.User != nil && ((len(note.To.OrderedMap) == 0 || len(note.To.OrderedMap) == 1 && note.To.Contains(r.User.ID)) && (len(note.CC.OrderedMap) == 0 || len(note.CC.OrderedMap) == 1 && note.CC.Contains(r.User.ID))) {
			w.Titlef("📟 Message from %s", author.PreferredUsername)
		} else if note.InReplyTo != "" {
			w.Titlef("💬 Reply by %s", author.PreferredUsername)
		} else if note.IsPublic() {
			w.Titlef("📣 Post by %s", author.PreferredUsername)
		} else {
			w.Titlef("🔔 Post by %s", author.PreferredUsername)
		}

		if group.Valid {
			r.PrintNote(w, &note, &author, &group.Actor, false, false, true, false)
		} else {
			r.PrintNote(w, &note, &author, nil, false, false, true, false)
		}

		if note.Type == ap.QuestionObject && note.VotersCount > 0 && offset == 0 {
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

		if count > 0 && offset >= h.Config.RepliesPerPage {
			w.Empty()
			w.Subtitlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+h.Config.RepliesPerPage)
		} else if count > 0 {
			w.Empty()
			w.Subtitle("💬 Replies")
		}
	}

	r.PrintNotes(w, replies, false, false)

	var originalPostExists int
	var threadHead sql.NullString
	if note.InReplyTo != "" {
		if err := r.QueryRow(`select exists (select 1 from notes where id = ?)`, note.InReplyTo).Scan(&originalPostExists); err != nil {
			r.Log.Warn("Failed to check if parent post exists", "error", err)
		}

		if err := r.QueryRow(`with recursive thread(id, parent, depth) as (select notes.id, notes.object->>'inReplyTo' as parent, 1 as depth from notes where id = ? union select notes.id, notes.object->>'inReplyTo' as parent, t.depth + 1 from thread t join notes on notes.id = t.parent) select id from thread order by depth desc limit 1`, note.InReplyTo).Scan(&threadHead); err != nil && errors.Is(err, sql.ErrNoRows) {
			r.Log.Debug("First post in thread is missing")
		} else if err != nil {
			r.Log.Warn("Failed to fetch first post in thread", "error", err)
		}
	}

	var threadDepth int
	if err := r.QueryRow(`with recursive thread(id, depth) as (select notes.id, 0 as depth from notes where id = ? union select notes.id, t.depth + 1 from thread t join notes on notes.object->>'inReplyTo' = t.id where t.depth <= 3) select max(thread.depth) from thread`, note.ID).Scan(&threadDepth); err != nil {
		r.Log.Warn("Failed to query thread depth", "error", err)
	}

	if originalPostExists == 1 || (threadHead.Valid && threadHead.String != note.ID && threadHead.String != note.InReplyTo) || threadDepth > 2 || offset > h.Config.RepliesPerPage || offset >= h.Config.RepliesPerPage || count == h.Config.RepliesPerPage {
		w.Separator()
	}

	if originalPostExists == 1 && r.User == nil {
		w.Link("/view/"+strings.TrimPrefix(note.InReplyTo, "https://"), "View parent post")
	} else if originalPostExists == 1 {
		w.Link("/users/view/"+strings.TrimPrefix(note.InReplyTo, "https://"), "View parent post")
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
