/*
Copyright 2023, 2024 Dima Krasner

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
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"strings"
)

func (h *Handler) thread(w text.Writer, r *Request, args ...string) {
	postID := "https://" + args[1]

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.Log.Info("Viewing thread", "post", postID)

	var threadHead sql.NullString
	if err := h.DB.QueryRowContext(
		r.Context,
		`with recursive thread(id, parent) as (select notes.id, notes.object->>'$.inReplyTo' as parent from notes where id = ? union all select notes.id, notes.object->>'$.inReplyTo' as parent from thread t join notes on notes.id = t.parent) select thread.id from thread where thread.parent is null limit 1`,
		postID,
	).Scan(&threadHead); err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Failed to fetch thread head", "error", err)
		w.Error()
		return
	}

	var rootAuthorID, rootAuthorType, rootAuthorUsername string
	var rootAuthorName sql.NullString
	if err := h.DB.QueryRowContext(
		r.Context,
		`select persons.id, persons.actor->>'$.type', persons.actor->>'$.preferredUsername', persons.actor->>'$.name' from notes join persons on persons.id = notes.author where notes.id = ?`,
		postID,
	).Scan(&rootAuthorID, &rootAuthorType, &rootAuthorUsername, &rootAuthorName); err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Failed to fetch thread root author", "error", err)
		w.Error()
		return
	}

	rows, err := h.DB.QueryContext(
		r.Context,
		`select thread.depth, thread.id, strftime('%Y-%m-%d', datetime(thread.inserted, 'unixepoch')), persons.actor->>'$.preferredUsername' from (with recursive thread(id, author, inserted, parent, depth, path) as (select notes.id, notes.author, notes.inserted, object->>'$.inReplyTo' as parent, 0 as depth, notes.inserted || notes.id as path from notes where id = $1 union all select notes.id, notes.author, notes.inserted, notes.object->>'$.inReplyTo', t.depth + 1, t.path || notes.inserted || notes.id from thread t join notes on notes.object->>'$.inReplyTo' = t.id) select thread.depth, thread.id, thread.author, thread.inserted, thread.path from thread order by thread.path limit $2 offset $3) thread join persons on persons.id = thread.author order by thread.path`,
		postID,
		h.Config.PostsPerPage,
		offset,
	)
	if err != nil {
		r.Log.Info("Failed to fetch thread", "post", postID, "error", err)
		w.Status(40, "Post not found")
		return
	}
	defer rows.Close()

	w.OK()

	var displayName string
	if rootAuthorName.Valid {
		displayName = h.getDisplayName(rootAuthorID, rootAuthorUsername, rootAuthorName.String, ap.ActorType(rootAuthorType))
	} else {
		displayName = h.getDisplayName(rootAuthorID, rootAuthorUsername, "", ap.ActorType(rootAuthorType))
	}

	if offset > 0 && offset >= h.Config.PostsPerPage {
		w.Titlef("🧵 Replies to %s (%d-%d)", displayName, offset, offset+h.Config.PostsPerPage)
	} else {
		w.Titlef("🧵 Replies to %s", displayName)
	}

	count := 0
	var firstNodeID string
	for rows.Next() {
		var node struct {
			Depth                            int
			PostID, Inserted, AuthorUserName string
		}

		if err := rows.Scan(
			&node.Depth,
			&node.PostID,
			&node.Inserted,
			&node.AuthorUserName,
		); err != nil {
			r.Log.Info("Failed to scan post", "post", postID, "error", err)
			continue
		}

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
			w.Link("/view/"+strings.TrimPrefix(node.PostID, "https://"), b.String())
		} else {
			w.Link("/users/view/"+strings.TrimPrefix(node.PostID, "https://"), b.String())
		}

		if count == 0 {
			firstNodeID = node.PostID
		}

		count++
	}

	if count == 0 {
		r.Log.Info("Failed to fetch any nodes in thread", "post", postID)
		w.Error()
		return
	}

	if (threadHead.Valid && count > 0 && threadHead.String != firstNodeID) || offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Separator()
	}

	if threadHead.Valid && count > 0 && threadHead.String != firstNodeID && r.User == nil {
		w.Link("/view/"+strings.TrimPrefix(threadHead.String, "https://"), "View first post in thread")
	} else if threadHead.Valid && count > 0 && threadHead.String != firstNodeID {
		w.Link("/users/view/"+strings.TrimPrefix(threadHead.String, "https://"), "View first post in thread")
	}

	if offset > h.Config.PostsPerPage {
		w.Link(r.URL.Path, "First page")
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}
}
