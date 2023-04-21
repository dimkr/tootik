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
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/text"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/view/[0-9a-f]{64}$`)] = withUserMenu(view)
	handlers[regexp.MustCompile(`^/view/[0-9a-f]{64}$`)] = withUserMenu(view)
}

func view(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.WithError(err).Info("Failed to parse query")
		w.Status(40, "Invalid query")
		return
	}

	r.Log.WithField("hash", hash).Info("Viewing post")

	id := ""
	noteString := ""
	if err := r.QueryRow(`select id, object from notes where hash = ?`, hash).Scan(&id, &noteString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.WithField("post", id).Info("Post was not found")
		w.Status(40, "Post not found")
		return
	} else if err != nil {
		r.Log.WithField("post", id).WithError(err).Info("Failed to find post")
		w.Error()
		return
	}

	note := ap.Object{}
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		w.Error()
		return
	}

	author, err := r.Resolve(note.AttributedTo)
	if err != nil {
		r.Log.WithField("post", id).WithError(err).Info("Failed to resolve post author")
		w.Error()
		return
	}

	rows, err := r.Query(`select replies.object, persons.actor from notes join notes replies on replies.object->>'inReplyTo' = notes.id left join persons on persons.id = replies.author where notes.hash = ? order by replies.inserted desc limit ? offset ?;`, hash, repliesPerPage, offset)
	if err != nil {
		r.Log.WithField("post", note.ID).WithError(err).Info("Failed to fetch replies")
		w.Error()
		return
	}
	defer rows.Close()

	replies := data.OrderedMap[string, sql.NullString]{}

	for rows.Next() {
		var replyString string
		var replierString sql.NullString
		if err := rows.Scan(&replyString, &replierString); err != nil {
			r.Log.WithError(err).Warn("Failed to scan reply")
			continue
		}

		replies.Store(replyString, replierString)
	}
	rows.Close()

	count := len(replies)

	authorDisplayName := getActorDisplayName(author)

	w.OK()

	if offset > 0 {
		w.Titlef("💬 Replies to %s (%d-%d)", authorDisplayName, offset, offset+repliesPerPage)
	} else {
		if r.User != nil && ((len(note.To.OrderedMap) == 0 || len(note.To.OrderedMap) == 1 && note.To.Contains(r.User.ID)) && (len(note.CC.OrderedMap) == 0 || len(note.CC.OrderedMap) == 1 && note.CC.Contains(r.User.ID))) {
			w.Titlef("📟 Message from %s", authorDisplayName)
		} else if note.InReplyTo != "" {
			w.Titlef("💬 Reply by %s", authorDisplayName)
		} else if note.IsPublic() {
			w.Titlef("📣 Post by %s", authorDisplayName)
		} else {
			w.Titlef("🔔 Post by %s", authorDisplayName)
		}

		printNote(w, r, &note, author, false, false, true)

		if count > 0 && offset >= repliesPerPage {
			w.Empty()
			w.Subtitlef("💬 Replies to %s (%d-%d)", authorDisplayName, offset, offset+repliesPerPage)
		} else if count > 0 {
			w.Empty()
			w.Subtitle("💬 Replies")
		}
	}

	printNotes(w, r, replies, true, false)

	if note.InReplyTo != "" || offset >= repliesPerPage || count == repliesPerPage {
		w.Separator()
	}

	if note.InReplyTo != "" && r.User == nil {
		w.Link(fmt.Sprintf("/view/%x", sha256.Sum256([]byte(note.InReplyTo))), "View original post")
	} else if note.InReplyTo != "" {
		w.Link(fmt.Sprintf("/users/view/%x", sha256.Sum256([]byte(note.InReplyTo))), "View original post")
	}

	if offset > repliesPerPage && r.User == nil {
		w.Link("/view/"+hash, "First page")
	} else if offset > repliesPerPage {
		w.Link("/users/view/"+hash, "First page")
	}

	if offset >= repliesPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/view/%s?%d", hash, offset-repliesPerPage), "Previous page (%d-%d)", offset-repliesPerPage, offset)
	} else if offset >= repliesPerPage {
		w.Linkf(fmt.Sprintf("/users/view/%s?%d", hash, offset-repliesPerPage), "Previous page (%d-%d)", offset-repliesPerPage, offset)
	}

	if count == repliesPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/view/%s?%d", hash, offset+repliesPerPage), "Next page (%d-%d)", offset+repliesPerPage, offset+2*repliesPerPage)
	} else if count == repliesPerPage {
		w.Linkf(fmt.Sprintf("/users/view/%s?%d", hash, offset+repliesPerPage), "Next page (%d-%d)", offset+repliesPerPage, offset+2*repliesPerPage)
	}
}
