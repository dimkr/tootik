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

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) doReply(w text.Writer, r *Request, args []string, quote bool, readInput inputFunc) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	postID := "https://" + args[1]

	var note ap.Object
	if err := h.DB.QueryRowContext(
		r.Context,
		`
		select json(notes.object) from notes
		join persons on persons.id = notes.author
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
							follows.accepted = 1 and
							follows.follower = $2
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
	).Scan(&note); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Post does not exist", "post", postID)
		w.Status(40, "Post not found")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find post by ID", "post", postID, "error", err)
		w.Error()
		return
	}

	to := ap.Audience{}
	cc := ap.Audience{}

	if quote {
		r.Log.Info("Quoting post", "post", note.ID)

		if !note.IsPublic() {
			r.Log.Warn("Cannot quote a non-public post", "post", postID)
			w.Status(40, "Cannot quote a non-public post")
			return
		}

		note.Quote = postID

		to.Add(note.AttributedTo)
		cc.Add(r.User.Followers)
		cc.Add(ap.Public)

		h.post(w, r, nil, nil, note.ID, to, cc, note.Audience, readInput)
		return
	}

	r.Log.Info("Replying to post", "post", note.ID)

	if note.AttributedTo == r.User.ID {
		to = note.To
		cc = note.CC
	} else if note.To.Contains(ap.Public) {
		to.Add(note.AttributedTo)
		to.Add(ap.Public)
		cc.Add(r.User.Followers)
	} else if note.CC.Contains(ap.Public) {
		to.Add(note.AttributedTo)
		cc.Add(r.User.Followers)
		cc.Add(ap.Public)
	} else {
		to.Add(note.AttributedTo)
		for id := range note.To.Keys() {
			cc.Add(id)
		}
		for id := range note.CC.Keys() {
			cc.Add(id)
		}
	}

	h.post(w, r, nil, &note, "", to, cc, note.Audience, readInput)
}

func (h *Handler) reply(w text.Writer, r *Request, args ...string) {
	h.doReply(w, r, args, false, func() (string, bool) {
		return readQuery(w, r, "Reply content")
	})
}

func (h *Handler) replyUpload(w text.Writer, r *Request, args ...string) {
	h.doReply(w, r, args, false, func() (string, bool) {
		return h.readBody(w, r, args[1:])
	})
}

func (h *Handler) quote(w text.Writer, r *Request, args ...string) {
	h.doReply(w, r, args, true, func() (string, bool) {
		return readQuery(w, r, "Reply content")
	})
}

func (h *Handler) quoteUpload(w text.Writer, r *Request, args ...string) {
	h.doReply(w, r, args, true, func() (string, bool) {
		return h.readBody(w, r, args[1:])
	})
}
