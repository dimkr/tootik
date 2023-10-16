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
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/outbox"
	"math"
	"net/url"
	"path/filepath"
	"time"
)

func edit(w text.Writer, r *request) {
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

	if len(content) > cfg.MaxPostsLength {
		w.Status(40, "Post is too long")
		return
	}

	hash := filepath.Base(r.URL.Path)

	var noteString string
	if err := r.QueryRow(`select object from notes where hash = ? and author = ?`, hash, r.User.ID).Scan(&noteString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Attempted to edit non-existing post", "hash", hash, "error", err)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch post to edit", "hash", hash, "error", err)
		w.Error()
		return
	}

	var note ap.Object
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		r.Log.Warn("Failed to unmarshal post to edit", "hash", hash, "error", err)
		w.Error()
		return
	}

	if note.Name != "" {
		r.Log.Warn("Cannot edit votes", "vote", note.ID)
		w.Status(40, "Cannot edit votes")
		return
	}

	var edits int
	if err := r.QueryRow(`select count(*) from outbox where activity->>'object.id' = ? and (activity->>'type' = 'Update' or activity->>'type' = 'Create')`, note.ID).Scan(&edits); err != nil {
		r.Log.Warn("Failed to count post edits", "hash", hash, "error", err)
		w.Error()
		return
	}

	lastEditTime := note.Published
	if note.Updated != nil && *note.Updated != (time.Time{}) {
		lastEditTime = *note.Updated
	}

	canEdit := lastEditTime.Add(time.Minute * time.Duration(math.Pow(4, float64(edits))))
	if time.Now().Before(canEdit) {
		r.Log.Warn("Throttled request to edit post", "note", note.ID, "can", canEdit)
		w.Status(40, "Please try again later")
		return
	}

	if err := outbox.Edit(r.Context, r.DB, &note, plain.ToHTML(content)); err != nil {
		r.Log.Error("Failed to update post", "note", note.ID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/view/%s", hash)
}
