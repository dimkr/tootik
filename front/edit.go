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
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"math"
	"net/url"
	"time"
	"unicode/utf8"
)

func (h *Handler) edit(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Post content")
		return
	}

	content, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		w.Status(40, "Bad input")
		return
	}

	if utf8.RuneCountInString(content) > h.Config.MaxPostsLength {
		w.Status(40, "Post is too long")
		return
	}

	postID := "https://" + args[1]

	var note ap.Object
	if err := r.QueryRow(`select object from notes where id = ? and author = ?`, postID, r.User.ID).Scan(&note); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Attempted to edit non-existing post", "post", postID, "error", err)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch post to edit", "post", postID, "error", err)
		w.Error()
		return
	}

	if note.Name != "" {
		r.Log.Warn("Cannot edit votes", "vote", note.ID)
		w.Status(40, "Cannot edit votes")
		return
	}

	var edits int
	if err := r.QueryRow(`select count(*) from outbox where activity->>'$.object.id' = ? and (activity->>'type' = 'Update' or activity->>'type' = 'Create')`, note.ID).Scan(&edits); err != nil {
		r.Log.Warn("Failed to count post edits", "post", postID, "error", err)
		w.Error()
		return
	}

	lastEditTime := note.Published
	if note.Updated != nil && *note.Updated != (ap.Time{}) {
		lastEditTime = *note.Updated
	}

	canEdit := lastEditTime.Add(h.Config.EditThrottleUnit * time.Duration(math.Pow(h.Config.EditThrottleFactor, float64(edits))))
	if time.Now().Before(canEdit) {
		r.Log.Warn("Throttled request to edit post", "note", note.ID, "can", canEdit)
		w.Status(40, "Please try again later")
		return
	}

	if note.InReplyTo == "" {
		h.post(w, r, &note, nil, note.To, note.CC, note.Audience, "Post content")
		return
	}

	var parent ap.Object
	if err := r.QueryRow(`select object from notes where id = ?`, note.InReplyTo).Scan(&parent); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Parent post does not exist", "parent", note.InReplyTo)
	} else if err != nil {
		r.Log.Warn("Failed to fetch parent post", "parent", note.InReplyTo, "error", err)
		w.Error()
		return
	}

	// the starting point is the original value of to and cc: recipients can be added but not removed when editing
	h.post(w, r, &note, &parent, note.To, note.CC, note.Audience, "Post content")
}
