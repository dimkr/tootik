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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"path/filepath"
	"strings"
)

type threadNode struct {
	Depth                            int
	PostID, Inserted, AuthorUserName string
}

func thread(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.Log.Info("Viewing thread", "hash", hash)

	var threadHead sql.NullString
	if err := r.QueryRow(`with recursive thread(id, parent) as (select notes.id, notes.object->>'inReplyTo' as parent from notes where hash = ? union select notes.id, notes.object->>'inReplyTo' as parent from thread t join notes on notes.id = t.parent) select thread.id from thread where thread.parent is null limit 1`, hash).Scan(&threadHead); err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Failed to fetch thread head", "error", err)
		w.Error()
		return
	}

	var rootAuthorID, rootAuthorType, rootAuthorUsername string
	var rootAuthorName sql.NullString
	if err := r.QueryRow(`select persons.id, persons.actor->>'type', persons.actor->>'preferredUsername', persons.actor->>'name' from notes join persons on persons.id = notes.author where notes.hash = ?`, hash).Scan(&rootAuthorID, &rootAuthorType, &rootAuthorUsername, &rootAuthorName); err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Failed to fetch thread root author", "error", err)
		w.Error()
		return
	}

	rows, err := r.Query(`select thread.depth, thread.id, strftime('%Y-%m-%d', datetime(thread.inserted, 'unixepoch')), persons.actor->>'preferredUsername' from (with recursive thread(id, author, inserted, parent, depth, path) as (select notes.id, notes.author, notes.inserted, object->>'inReplyTo' as parent, 0 as depth, notes.id as path from notes where hash = $1 union select notes.id, notes.author, notes.inserted, notes.object->>'inReplyTo', t.depth + 1, t.path || ';' || notes.id from thread t join notes on notes.object->>'inReplyTo' = t.id where t.depth <= 5) select thread.depth, thread.id, thread.author, thread.inserted, thread.path from thread order by thread.path limit $2 offset $3) thread join persons on persons.id = thread.author order by thread.path`, hash, postsPerPage, offset)
	if err != nil {
		r.Log.Info("Failed to fetch thread", "hash", hash, "error", err)
		w.Status(40, "Post not found")
		return
	}
	defer rows.Close()

	count := 0
	nodes := make([]threadNode, postsPerPage)

	for rows.Next() {
		if err := rows.Scan(
			&nodes[count].Depth,
			&nodes[count].PostID,
			&nodes[count].Inserted,
			&nodes[count].AuthorUserName,
		); err != nil {
			r.Log.Info("Failed to scan post", "hash", hash, "error", err)
			continue
		}

		count++
	}
	rows.Close()

	if count == 0 {
		r.Log.Info("Failed to fetch any nodes in thread", "hash", hash)
		w.Error()
		return
	}

	w.OK()

	var displayName string
	if rootAuthorName.Valid {
		displayName = getDisplayName(rootAuthorID, rootAuthorUsername, rootAuthorName.String, ap.ActorType(rootAuthorType), r.Log)
	} else {
		displayName = getDisplayName(rootAuthorID, rootAuthorUsername, "", ap.ActorType(rootAuthorType), r.Log)
	}
	w.Titlef("🧵 Replies to %s", displayName)

	for _, node := range nodes[:count] {
		var b strings.Builder
		b.WriteString(node.Inserted)
		b.WriteByte(' ')
		if node.Depth > 0 {
			for i := 0; i < node.Depth; i++ {
				b.WriteRune('·')
			}
			b.WriteByte(' ')
		}
		b.WriteString(node.AuthorUserName)

		if r.User == nil {
			w.Link(fmt.Sprintf("/view/%x", sha256.Sum256([]byte(node.PostID))), b.String())
		} else {
			w.Link(fmt.Sprintf("/users/view/%x", sha256.Sum256([]byte(node.PostID))), b.String())
		}
	}

	if (threadHead.Valid && count > 0 && threadHead.String != nodes[0].PostID) || offset >= postsPerPage || count == postsPerPage {
		w.Separator()
	}

	if threadHead.Valid && count > 0 && threadHead.String != nodes[0].PostID && r.User == nil {
		w.Link(fmt.Sprintf("/view/%x", sha256.Sum256([]byte(threadHead.String))), "View first post in thread")
	} else if threadHead.Valid && count > 0 && threadHead.String != nodes[0].PostID {
		w.Link(fmt.Sprintf("/users/view/%x", sha256.Sum256([]byte(threadHead.String))), "View first post in thread")
	}

	if offset > postsPerPage && r.User == nil {
		w.Link("/thread/"+hash, "First page")
	} else if offset > postsPerPage {
		w.Link("/users/thread/"+hash, "First page")
	}

	if offset >= postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/thread/%s?%d", hash, offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		w.Linkf(fmt.Sprintf("/users/thread/%s?%d", hash, offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	}

	if count == postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/thread/%s?%d", hash, offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	} else if count == postsPerPage {
		w.Linkf(fmt.Sprintf("/users/thread/%s?%d", hash, offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	}
}
