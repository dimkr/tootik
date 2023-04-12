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

package gem

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"io"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/view/[0-9a-f]{64}$`)] = withUserMenu(view)
	handlers[regexp.MustCompile(`^/view/[0-9a-f]{64}$`)] = withUserMenu(view)
}

func view(w io.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.WithError(err).Info("Failed to parse query")
		w.Write([]byte("40 Invalid query\r\n"))
		return
	}

	r.Log.WithField("hash", hash).Info("Viewing post")

	id := ""
	noteString := ""
	if err := r.QueryRow(`select id, object from notes where hash = ?`, hash).Scan(&id, &noteString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.WithField("post", id).Info("Post was not found")
		w.Write([]byte("40 Post not found\r\n"))
		return
	} else if err != nil {
		r.Log.WithField("post", id).WithError(err).Info("Failed to find post")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	note := ap.Object{}
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		w.Write([]byte("40 Error\r\n"))
		return
	}

	author, err := r.Resolve(note.AttributedTo)
	if err != nil {
		r.Log.WithField("post", id).WithError(err).Info("Failed to resolve post author")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	rows, err := r.Query(`select replies.object, persons.actor from notes join notes replies on replies.object->>'inReplyTo' = notes.id left join persons on persons.id = replies.author where notes.hash = ? order by replies.inserted desc limit ? offset ?;`, hash, repliesPerPage, offset)
	if err != nil {
		r.Log.WithField("post", note.ID).WithError(err).Info("Failed to fetch replies")
		w.Write([]byte("40 Error\r\n"))
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

	w.Write([]byte("20 text/gemini\r\n"))

	if offset > 0 {
		fmt.Fprintf(w, "# 💬 Replies to %s (%d-%d)\n\n", authorDisplayName, offset, offset+repliesPerPage)
	} else {
		if note.InReplyTo != "" {
			fmt.Fprintf(w, "# 💬 Reply by %s\n", authorDisplayName)
		} else if note.IsPublic() {
			fmt.Fprintf(w, "# 📣 Post by %s\n", authorDisplayName)
		} else {
			fmt.Fprintf(w, "# 🔔 Post by %s\n", authorDisplayName)
		}

		w.Write([]byte{'\n'})

		printNote(w, r, &note, author, false, false, true)

		if count > 0 && offset >= repliesPerPage {
			fmt.Fprintf(w, "\n## 💬 Replies to %s (%d-%d)\n\n", authorDisplayName, offset, offset+repliesPerPage)
		} else if count > 0 {
			w.Write([]byte("\n## 💬 Replies\n\n"))
		}
	}

	printNotes(w, r, replies, true, false)

	if note.InReplyTo != "" || offset >= repliesPerPage || count == repliesPerPage {
		w.Write([]byte("────\n\n"))
	}

	if note.InReplyTo != "" && r.User == nil {
		fmt.Fprintf(w, "=> /view/%x View original post\n", sha256.Sum256([]byte(note.InReplyTo)))
	} else if note.InReplyTo != "" {
		fmt.Fprintf(w, "=> /users/view/%x View original post\n", sha256.Sum256([]byte(note.InReplyTo)))
	}

	if offset > repliesPerPage && r.User == nil {
		fmt.Fprintf(w, "=> /view/%s First page\n", hash)
	} else if offset > repliesPerPage {
		fmt.Fprintf(w, "=> /users/view/%s First page\n", hash)
	}

	if offset >= repliesPerPage && r.User == nil {
		fmt.Fprintf(w, "=> /view/%s?%d Previous page (%d-%d)\n", hash, offset-repliesPerPage, offset-repliesPerPage, offset)
	} else if offset >= repliesPerPage {
		fmt.Fprintf(w, "=> /users/view/%s?%d Previous page (%d-%d)\n", hash, offset-repliesPerPage, offset-repliesPerPage, offset)
	}

	if count == repliesPerPage && r.User == nil {
		fmt.Fprintf(w, "=> /view/%s?%d Next page (%d-%d)\n", hash, offset+repliesPerPage, offset+repliesPerPage, offset+2*repliesPerPage)
	} else if count == repliesPerPage {
		fmt.Fprintf(w, "=> /users/view/%s?%d Next page (%d-%d)\n", hash, offset+repliesPerPage, offset+repliesPerPage, offset+2*repliesPerPage)
	}
}
