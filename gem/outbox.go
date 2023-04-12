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
	log "github.com/sirupsen/logrus"
	"io"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/outbox/[0-9a-f]{64}$`)] = withUserMenu(outbox)
	handlers[regexp.MustCompile(`^/outbox/[0-9a-f]{64}$`)] = withUserMenu(outbox)
}

func printNotes(w io.Writer, r *request, rows data.OrderedMap[string, sql.NullString], printAuthor, printParentAuthor bool) {
	rows.Range(func(noteString string, actorString sql.NullString) bool {
		note := ap.Object{}
		if err := json.Unmarshal([]byte(noteString), &note); err != nil {
			r.Log.WithError(err).Warn("Failed to unmarshal post")
			return true
		}

		if note.Type != ap.NoteObject {
			r.Log.WithField("type", note.Type).Warn("Post is note a note")
			return true
		}

		if actorString.Valid && actorString.String != "" {
			author := ap.Actor{}
			if err := json.Unmarshal([]byte(actorString.String), &author); err != nil {
				r.Log.WithError(err).Warn("Failed to unmarshal post author")
				return true
			}

			printNote(w, r, &note, &author, true, printAuthor, printParentAuthor)
		} else {
			if author, err := r.Resolve(note.AttributedTo); err != nil {
				r.Log.WithFields(log.Fields{"note": note.ID, "author": note.AttributedTo}).WithError(err).Warn("Failed to resolve post author")
				return true
			} else {
				printNote(w, r, &note, author, true, printAuthor, printParentAuthor)
			}
		}
		w.Write([]byte{'\n'})

		return true
	})
}

func outbox(w io.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	var actorID, actorString string
	if err := r.QueryRow(`select id, actor from persons where hash = ?`, hash).Scan(&actorID, &actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.WithField("hash", hash).Info("Person was not found")
		w.Write([]byte("40 User not found\r\n"))
		return
	} else if err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to find person by hash")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	actor := ap.Actor{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to unmarshal actor")
		w.Write([]byte("40 Error\r\n"))
	}

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.WithField("url", r.URL.String()).WithError(err).Info("Failed to parse query")
		w.Write([]byte("40 Invalid query\r\n"))
		return
	}

	r.Log.WithFields(log.Fields{"actor": actorID, "offset": offset}).Info("Viewing outbox")

	rows, err := r.Query(`select object from notes where author = ? order by inserted desc limit ? offset ?`, actorID, postsPerPage, offset)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to fetch posts")
		w.Write([]byte("40 Error\r\n"))
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, sql.NullString]{}

	for rows.Next() {
		noteString := ""
		if err := rows.Scan(&noteString); err != nil {
			r.Log.WithError(err).Warn("Failed to scan post")
			continue
		}

		notes.Store(noteString, sql.NullString{})
	}
	rows.Close()

	count := len(notes)

	w.Write([]byte("20 text/gemini\r\n"))

	displayName := getActorDisplayName(&actor)

	summary := ""
	var links []string
	if offset == 0 {
		summary, links = getQuoteAndLinks(actor.Summary, -1)
	}

	if offset >= postsPerPage || count == postsPerPage {
		fmt.Fprintf(w, "# %s (%d-%d)\n\n", displayName, offset, offset+postsPerPage)
	} else {
		fmt.Fprintf(w, "# %s\n\n", displayName)
	}

	if summary != "" {
		w.Write([]byte(summary))
		for _, link := range links {
			fmt.Fprintf(w, "=> %s\n", link)
		}
		w.Write([]byte("\n────\n\n"))
	}

	printNotes(w, r, notes, false, true)

	if offset >= postsPerPage || count == postsPerPage {
		w.Write([]byte("────\n\n"))
	}

	if offset >= postsPerPage && r.User == nil {
		fmt.Fprintf(w, "=> /outbox/%s?%d Previous page (%d-%d)\n", hash, offset-postsPerPage, offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		fmt.Fprintf(w, "=> /users/outbox/%s?%d Previous page (%d-%d)\n", hash, offset-postsPerPage, offset-postsPerPage, offset)
	}

	if count == postsPerPage && r.User == nil {
		fmt.Fprintf(w, "=> /outbox/%s?%d Next page (%d-%d)\n", hash, offset+postsPerPage, offset+postsPerPage, offset+2*postsPerPage)
	} else if count == postsPerPage {
		fmt.Fprintf(w, "=> /users/outbox/%s?%d Next page (%d-%d)\n", hash, offset+postsPerPage, offset+postsPerPage, offset+2*postsPerPage)
	}

	if r.User != nil && actorID != r.User.ID {
		var followID string
		err := r.QueryRow(`select id from follows where follower = ? and followed = ?`, r.User.ID, actorID).Scan(&followID)
		if err != nil && errors.Is(err, sql.ErrNoRows) {
			w.Write([]byte("────\n\n"))
			fmt.Fprintf(w, "=> /users/follow/%x ⚡ Follow %s\n", sha256.Sum256([]byte(actorID)), displayName)
		} else if err != nil {
			r.Log.WithField("followed", actorID).WithError(err).Warn("Failed to check if user is followed")
		} else {
			w.Write([]byte("────\n\n"))
			fmt.Fprintf(w, "=> /users/unfollow/%x 🔌 Unfollow %s\n", sha256.Sum256([]byte(actorID)), displayName)
		}
	}
}
