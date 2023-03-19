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
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/go-ap/activitypub"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/outbox(/[a-f0-9]+){0,1}$`)] = withUserMenu(outbox)
	handlers[regexp.MustCompile(`^/outbox(/[a-f0-9]+){0,1}$`)] = withUserMenu(outbox)
}

func printNotes(ctx context.Context, w io.Writer, rows *sql.Rows, db *sql.DB, viewer *data.Object) {
	for rows.Next() {
		actor := ""
		s := ""
		if err := rows.Scan(&actor, &s); err != nil {
			log.WithError(err).Warn("Failed to scan post")
			continue
		}

		note := activitypub.Object{}
		if err := json.Unmarshal([]byte(s), &note); err != nil {
			log.WithError(err).Warn("Failed to unmarshal post")
			continue
		}

		if note.Type != activitypub.NoteType {
			log.WithField("type", note.Type).Warn("Post is note a note")
			continue
		}

		printNote(ctx, db, w, actor, &note, viewer, true)
		w.Write([]byte{'\n'})
	}
}

func outbox(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	hash := filepath.Base(requestUrl.Path)

	if hash == "" {
		log.Warn("Received outbox request without user hash")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	u, err := data.Objects.GetByHash(hash, db)
	if err != nil {
		log.WithField("hash", hash).WithError(err).Warn("Failed to find person by hash")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	offset, err := getOffset(requestUrl)
	if err != nil {
		log.WithField("url", requestUrl.String()).WithError(err).Info("Failed to parse query")
		w.Write([]byte("40 Invalid query\r\n"))
		return
	}

	log.WithFields(log.Fields{"user": u.ID, "offset": offset}).Info("Viewing outbox")

	person, err := fed.Resolve(ctx, db, user, u.ID)
	if err != nil {
		log.WithField("user", u.ID).WithError(err).Warn("Failed to find user")
		w.Write([]byte("40 Error\r\n"))
		return
	}
	id := string(person.ID.GetLink())

	rows, err := db.Query(`select actor, object from objects where type = 'Note' and Actor = ? order by inserted desc limit ? offset ?`, id, postsPerPage, offset)
	if err != nil {
		log.WithField("user", id).WithError(err).Warn("Failed to fetch posts")
		w.Write([]byte("40 Error\r\n"))
		return
	}
	defer rows.Close()

	w.Write([]byte("20 text/gemini\r\n"))

	displayName := getActorDisplayName(person)

	summary := ""
	if offset > 0 {
		summary = getNativeLanguageValue(person.Summary)
	}

	if summary == "" {
		fmt.Fprintf(w, "# %s (%d-%d)\n\n", displayName, offset, offset+postsPerPage)
	} else {
		fmt.Fprintf(w, "# %s (%d-%d)\n\n%s\n", displayName, offset, offset+postsPerPage, summary)
		if summary[len(summary)-1] != '\n' {
			w.Write([]byte{'\n'})
		}
	}

	printNotes(ctx, w, rows, db, user)

	if offset >= postsPerPage && user == nil {
		fmt.Fprintf(w, "=> /outbox/%s?%d ⬅️ Previous page (%d-%d)\n", hash, offset-postsPerPage, offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		fmt.Fprintf(w, "=> /users/outbox/%s?%d ⬅️ Previous page (%d-%d)\n", hash, offset-postsPerPage, offset-postsPerPage, offset)
	}

	if user == nil {
		fmt.Fprintf(w, "=> /outbox/%s?%d ➡️ Next page (%d-%d)\n", hash, offset+postsPerPage, offset+postsPerPage, offset+2*postsPerPage)
	} else {
		fmt.Fprintf(w, "=> /users/outbox/%s?%d ➡️ Next page (%d-%d)\n", hash, offset+postsPerPage, offset+postsPerPage, offset+2*postsPerPage)
	}

	if user != nil && id != user.ID {
		preferredUsername := getNativeLanguageValue(person.PreferredUsername)

		var followID string
		err := db.QueryRow(`select id from objects where type = 'Follow' and actor = ? and object = ?`, user.ID, id).Scan(&followID)
		if err != nil && errors.Is(err, sql.ErrNoRows) {
			w.Write([]byte("────\n\n"))
			fmt.Fprintf(w, "=> /users/follow?%s 🙆 Follow %s\n", url.QueryEscape(id), preferredUsername)
		} else if err != nil {
			log.WithFields(log.Fields{"user": user.ID, "followed": id}).WithError(err).Warn("Failed to check if user is followed")
		} else {
			w.Write([]byte("────\n\n"))
			fmt.Fprintf(w, "=> /users/unfollow?%s 🙅 Unfollow %s\n", url.QueryEscape(id), preferredUsername)
		}
	}
}
