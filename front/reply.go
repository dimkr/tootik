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
	"encoding/json"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/text"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/reply/[0-9a-f]{64}`)] = reply
}

func reply(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	var noteString string
	if err := r.QueryRow(`select object from notes where hash = ?`, hash).Scan(&noteString); err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to find post by hash")
		w.Error()
		return
	}

	note := ap.Object{}
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		r.Log.WithField("post", note.ID).WithError(err).Warn("Failed to unmarshal post")
		w.Error()
		return
	}

	r.Log.WithField("post", note.ID).Info("Replying to post")

	to := ap.Audience{}
	cc := ap.Audience{}

	if note.AttributedTo == r.User.ID && note.IsPublic() {
		to.Add(ap.Public)
	} else if note.AttributedTo == r.User.ID && !note.IsPublic() {
		to.Add(r.User.Followers)
	} else if note.AttributedTo != r.User.ID && note.IsPublic() {
		to.Add(note.AttributedTo)
		cc.Add(ap.Public)
	} else {
		to.Add(note.AttributedTo)
		cc.Add(r.User.Followers)
	}

	post(w, r, &note, to, cc)
}
