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
	"github.com/dimkr/tootik/text"
	"path/filepath"
)

func reply(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	var noteString string
	if err := r.QueryRow(`select object from notes where hash = ?`, hash).Scan(&noteString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Post does not exist", "hash", hash)
		w.Status(40, "Post does not exist")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find post by hash", "hash", hash, "error", err)
		w.Error()
		return
	}

	note := ap.Object{}
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		r.Log.Warn("Failed to unmarshal post", "post", note.ID, "error", err)
		w.Error()
		return
	}

	r.Log.Info("Replying to post", "post", note.ID)

	to := ap.Audience{}
	cc := ap.Audience{}

	if note.AttributedTo == r.User.ID {
		to = note.To
		cc = note.CC
	} else if (len(note.To.OrderedMap) == 0 || len(note.To.OrderedMap) == 1 && note.To.Contains(r.User.ID)) && (len(note.CC.OrderedMap) == 0 || len(note.CC.OrderedMap) == 1 && note.CC.Contains(r.User.ID)) {
		to.Add(note.AttributedTo)
	} else if note.IsPublic() {
		to.Add(note.AttributedTo)
		cc.Add(r.User.Followers)
		cc.Add(ap.Public)
	} else if !note.IsPublic() {
		to.Add(note.AttributedTo)
		cc.Add(r.User.Followers)
	} else {
		r.Log.Error("Post audience is invalid", "post", note.ID)
		w.Error()
		return
	}

	post(w, r, &note, to, cc, "Reply content")
}
