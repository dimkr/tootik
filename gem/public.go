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
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/public$`)] = withCache(withUserMenu(public), time.Minute*5)
	handlers[regexp.MustCompile(`^/users/public$`)] = withCache(withUserMenu(public), time.Minute*5)
}

func printPublicPosts(ctx context.Context, w io.Writer, requestUrl *url.URL, user *data.Object, db *sql.DB) error {
	offset, err := getOffset(requestUrl)
	if err != nil {
		return fmt.Errorf("Failed to parse query: %w", err)
	}

	since := time.Now().Add(-time.Hour * 24).Unix()

	notes, err := db.Query(`select actor, object from objects where type = 'Note' and inserted >= ? and (exists (select 1 from json_each(object->'to') where value = 'https://www.w3.org/ns/activitystreams#Public' or value = 'Public' or value = 'as:Public') or exists (select 1 from json_each(object->'cc') where value = 'https://www.w3.org/ns/activitystreams#Public' or value = 'Public' or value = 'as:Public')) order by inserted desc limit ? offset ?;`, since, postsPerPage, offset)
	if err != nil {
		return fmt.Errorf("Failed to fetch notes: %w", err)
	}
	defer notes.Close()

	fmt.Fprintf(w, "# 🏙️ Latest Public Posts (%d-%d)\n\n", offset, offset+postsPerPage)

	printNotes(ctx, w, notes, db, user)

	w.Write([]byte("────\n\n"))

	if offset >= postsPerPage && user == nil {
		fmt.Fprintf(w, "=> /public?%d ⬅️ Previous page (%d-%d)\n", offset-postsPerPage, offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		fmt.Fprintf(w, "=> /users/public?%d ⬅️ Previous page (%d-%d)\n", offset-postsPerPage, offset-postsPerPage, offset)
	}

	if user == nil {
		fmt.Fprintf(w, "=> /public?%d ➡️ Next page (%d-%d)\n", offset+postsPerPage, offset+postsPerPage, offset+2*postsPerPage)
	} else {
		fmt.Fprintf(w, "=> /users/public?%d ➡️ Next page (%d-%d)\n", offset+postsPerPage, offset+postsPerPage, offset+2*postsPerPage)
	}

	return nil
}

func public(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	w.Write([]byte("20 text/gemini\r\n"))

	if user == nil {
		w.Write([]byte(logo))
	}

	if err := printPublicPosts(ctx, w, requestUrl, user, db); err != nil {
		log.WithError(err).Info("Failed to fetch public notes")
	}
}

func home(ctx context.Context, conn io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	if user != nil {
		conn.Write([]byte("30 /users\r\n"))
		return
	}

	var buf bytes.Buffer
	buf.Write([]byte(logo))
	fmt.Fprintf(&buf, "# %s\n\n", cfg.Domain)
	fmt.Fprintf(&buf, "Welcome! %s is a federated nanoblogging service.\n\n", cfg.Domain)
	buf.Write([]byte("WARNING: this is alpha-grade stuff that might eat your data or expose private data. This service comes with absolutely no warranty.\n\n"))

	if err := printPublicPosts(ctx, &buf, requestUrl, user, db); err != nil {
		log.WithError(err).Info("Failed to fetch public notes")
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	writeUserMenu(&buf, user)

	conn.Write([]byte("20 text/gemini\r\n"))
	conn.Write(buf.Bytes())
}
