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
	"fmt"
	"github.com/dimkr/tootik/data"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile("^/users$")] = withUserMenu(inbox)
}

func inbox(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	if user == nil {
		w.Write([]byte("61 Peer certificate is required\r\n"))
		return
	}

	offset, err := getOffset(requestUrl)
	if err != nil {
		log.WithField("url", requestUrl.String()).WithError(err).Info("Failed to parse query")
		w.Write([]byte("40 Invalid query\r\n"))
		return
	}

	log.WithFields(log.Fields{"user": user.ID, "offset": offset}).Info("Viewing inbox")

	rows, err := db.Query(`select actor, object from objects where type = 'Note' and actor in (select object from objects where type = 'Follow' and actor = ?) order by inserted desc limit ? offset ?;`, user.ID, postsPerPage, offset)
	if err != nil {
		log.WithField("user", user.ID).WithError(err).Warn("Failed to fetch posts")
		w.Write([]byte("40 Error\r\n"))
		return
	}
	defer rows.Close()

	w.Write([]byte("20 text/gemini\r\n"))
	fmt.Fprintf(w, "# 🗐 My inbox (%d-%d)\n\n", offset, offset+postsPerPage)

	printNotes(ctx, w, rows, db, user)

	w.Write([]byte("────\n\n"))
	if offset >= postsPerPage {
		fmt.Fprintf(w, "=> /users?%d ⬅️ Previous page (%d-%d)\n", offset-postsPerPage, offset-postsPerPage, offset)
	}
	fmt.Fprintf(w, "=> /users?%d ➡️ Next page (%d-%d)\n", offset+postsPerPage, offset+postsPerPage, offset+2*postsPerPage)
}
