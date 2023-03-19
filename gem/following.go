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
	"github.com/dimkr/tootik/fed"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^(/users/following$|/users/following\?.+)`)] = withUserMenu(following)
}

func following(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	if user == nil {
		w.Write([]byte("30 /users\r\n"))
		return
	}

	rows, err := db.Query(`select object from objects where type = 'Follow' and actor = ? order by inserted desc`, user.ID)
	if err != nil {
		log.WithField("follower", user.ID).WithError(err).Warn("Failed to list followed users")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	w.Write([]byte("20 text/gemini\r\n"))
	w.Write([]byte("# 🙆 Followed Users\n\n"))

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			log.WithField("follower", user.ID).WithError(err).Warn("Failed to list a followed user")
			continue
		}

		followed, err := fed.Resolve(ctx, db, user, id)
		if err != nil {
			log.WithFields(log.Fields{"follower": user.ID, "followed": id}).WithError(err).Warn("Failed to list a followed user")
			continue
		}

		followedID := string(followed.ID.GetLink())
		displayName := getActorDisplayName(followed)

		fmt.Fprintf(w, "=> /users/outbox/%x %s\n", sha256.Sum256([]byte(followedID)), displayName)
	}
}
