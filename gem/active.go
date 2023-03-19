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
	"crypto/sha256"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/data"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/users/active$`)] = withCache(withUserMenu(active), time.Hour)
	handlers[regexp.MustCompile(`^/active$`)] = withCache(withUserMenu(active), time.Hour)
}

func active(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	since := time.Now().Add(-time.Hour * 24 * 7).Unix()

	authors, err := db.Query(`select persons.id, persons.object->>'preferredUsername', persons.object->>'name', max(notes.inserted) as lastActivity, count(*) as count from objects notes left join objects persons on notes.type = 'Note' and notes.inserted >= ? and notes.actor = persons.id where persons.id is not null group by persons.id order by lastActivity desc, count desc;`, since)
	if err != nil {
		log.WithError(err).Warn("Failed to list active users")
		w.Write([]byte("40 Error\r\n"))
		return
	}
	defer authors.Close()

	w.Write([]byte("20 text/gemini\r\n"))
	w.Write([]byte("# 👥 Active Users\n\n"))

	w.Write([]byte("Users who posted in the last week:\n\n"))

	for authors.Next() {
		var authorID, preferredUsername string
		var name sql.NullString
		var lastInsert, count int64
		if err := authors.Scan(&authorID, &preferredUsername, &name, &lastInsert, &count); err != nil {
			log.WithError(err).Warn("Failed to fetch an author")
			continue
		}

		nameIfValid := ""
		if name.Valid && name.String != "" {
			nameIfValid = name.String
		}

		displayName := getDisplayName(authorID, preferredUsername, nameIfValid)

		if user == nil {
			fmt.Fprintf(w, "=> /outbox/%x %s %s (%d)\n", sha256.Sum256([]byte(authorID)), time.Unix(lastInsert, 0).Format(time.DateOnly), displayName, count)
		} else {
			fmt.Fprintf(w, "=> /users/outbox/%x %s %s (%d)\n", sha256.Sum256([]byte(authorID)), time.Unix(lastInsert, 0).Format(time.DateOnly), displayName, count)
		}
	}
}
