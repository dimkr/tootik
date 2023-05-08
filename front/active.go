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
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/text"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/users/active$`)] = withCache(withUserMenu(active), time.Hour)
	handlers[regexp.MustCompile(`^/active$`)] = withCache(withUserMenu(active), time.Hour)
}

func active(w text.Writer, r *request) {
	since := time.Now().Add(-time.Hour * 24 * 7).Unix()

	authors, err := r.Query(`select persons.id, persons.actor->>'type', persons.actor->>'preferredUsername', persons.actor->>'name', max(notes.inserted) as last, count(*) as count from notes left join persons on notes.inserted >= ? and notes.author = persons.id where persons.id is not null group by persons.id order by last desc, count desc;`, since)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to list active users")
		w.Error()
		return
	}
	defer authors.Close()

	w.OK()
	w.Title("🐾 Active Users")

	w.Text("Users who posted in the last week:")
	w.Empty()

	for authors.Next() {
		var authorID, authorType, preferredUsername string
		var name sql.NullString
		var lastInsert, count int64
		if err := authors.Scan(&authorID, &authorType, &preferredUsername, &name, &lastInsert, &count); err != nil {
			r.Log.WithError(err).Warn("Failed to fetch an author")
			continue
		}

		nameIfValid := ""
		if name.Valid && name.String != "" {
			nameIfValid = name.String
		}

		displayName := getDisplayName(authorID, preferredUsername, nameIfValid, ap.ActorType(authorType))

		w.Linkf(fmt.Sprintf("%s/outbox/%x", r.AuthPrefix, sha256.Sum256([]byte(authorID))), "%s %s (%d)", time.Unix(lastInsert, 0).Format(time.DateOnly), displayName, count)
	}
}
