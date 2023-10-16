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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
	"path/filepath"
)

func view(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.Log.Info("Viewing post", "hash", hash)

	var noteString, authorString string
	var groupString sql.NullString
	if err := r.QueryRow(`select notes.object, persons.actor, groups.actor from notes join persons on persons.id = notes.author left join (select id, actor from persons where actor->>'type' = 'Group') groups on groups.id = notes.groupid where notes.hash = ?`, hash).Scan(&noteString, &authorString, &groupString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Info("Post was not found", "hash", hash)
		w.Status(40, "Post not found")
		return
	} else if err != nil {
		r.Log.Info("Failed to find post", "hash", hash, "error", err)
		w.Error()
		return
	}

	note := ap.Object{}
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		r.Log.Info("Failed to unmarshal post", "hash", hash, "error", err)
		w.Error()
		return
	}

	r.AddLogContext("post", note.ID)

	author := ap.Actor{}
	if err := json.Unmarshal([]byte(authorString), &author); err != nil {
		r.Log.Info("Failed to unmarshal post author", "error", err)
		w.Error()
		return
	}

	group := ap.Actor{}
	if groupString.Valid {
		if err := json.Unmarshal([]byte(groupString.String), &group); err != nil {
			r.Log.Info("Failed to unmarshal post group", "error", err)
			w.Error()
			return
		}
	}

	rows, err := r.Query(`select replies.object, persons.actor from notes join notes replies on replies.object->>'inReplyTo' = notes.id left join persons on persons.id = replies.author where notes.hash = ? order by replies.inserted desc limit ? offset ?;`, hash, repliesPerPage, offset)
	if err != nil {
		r.Log.Info("Failed to fetch replies", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	replies := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		var replyString string
		var meta noteMetadata
		if err := rows.Scan(&replyString, &meta.Author); err != nil {
			r.Log.Warn("Failed to scan reply", "error", err)
			continue
		}

		replies.Store(replyString, meta)
	}
	rows.Close()

	count := len(replies)

	w.OK()

	if offset > 0 {
		w.Titlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+repliesPerPage)
	} else {
		if r.User != nil && ((len(note.To.OrderedMap) == 0 || len(note.To.OrderedMap) == 1 && note.To.Contains(r.User.ID)) && (len(note.CC.OrderedMap) == 0 || len(note.CC.OrderedMap) == 1 && note.CC.Contains(r.User.ID))) {
			w.Titlef("📟 Message from %s", author.PreferredUsername)
		} else if note.InReplyTo != "" {
			w.Titlef("💬 Reply by %s", author.PreferredUsername)
		} else if note.IsPublic() {
			w.Titlef("📣 Post by %s", author.PreferredUsername)
		} else {
			w.Titlef("🔔 Post by %s", author.PreferredUsername)
		}

		if groupString.Valid {
			r.PrintNote(w, &note, &author, &group, false, false, true, false)
		} else {
			r.PrintNote(w, &note, &author, nil, false, false, true, false)
		}

		if note.Type == ap.QuestionObject && note.VotersCount > 0 && offset == 0 {
			options := note.OneOf
			if len(options) == 0 {
				options = note.AnyOf
			}

			if len(options) > 0 {
				w.Empty()

				if note.VotersCount == 1 {
					w.Subtitle("📊 Results (one voter)")
				} else {
					w.Subtitlef("📊 Results (%d voters)", note.VotersCount)
				}

				labels := make([]string, 0, len(options))
				votes := make([]int64, 0, len(options))

				for _, option := range options {
					labels = append(labels, option.Name)
					votes = append(votes, option.Replies.TotalItems)
				}

				w.Raw("Results graph", graph.Bars(labels, votes))
			}
		}

		if count > 0 && offset >= repliesPerPage {
			w.Empty()
			w.Subtitlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+repliesPerPage)
		} else if count > 0 {
			w.Empty()
			w.Subtitle("💬 Replies")
		}
	}

	r.PrintNotes(w, replies, true, false)

	var originalPostExists int
	if note.InReplyTo != "" {
		if err := r.QueryRow(`select exists (select 1 from notes where id = ?)`, note.InReplyTo).Scan(&originalPostExists); err != nil {
			r.Log.Warn("Failed to check if original post exists", "error", err)
		}
	}

	if originalPostExists == 1 || offset >= repliesPerPage || count == repliesPerPage {
		w.Separator()
	}

	if originalPostExists == 1 && r.User == nil {
		w.Link(fmt.Sprintf("/view/%x", sha256.Sum256([]byte(note.InReplyTo))), "View original post")
	} else if originalPostExists == 1 {
		w.Link(fmt.Sprintf("/users/view/%x", sha256.Sum256([]byte(note.InReplyTo))), "View original post")
	}

	if offset > repliesPerPage && r.User == nil {
		w.Link("/view/"+hash, "First page")
	} else if offset > repliesPerPage {
		w.Link("/users/view/"+hash, "First page")
	}

	if offset >= repliesPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/view/%s?%d", hash, offset-repliesPerPage), "Previous page (%d-%d)", offset-repliesPerPage, offset)
	} else if offset >= repliesPerPage {
		w.Linkf(fmt.Sprintf("/users/view/%s?%d", hash, offset-repliesPerPage), "Previous page (%d-%d)", offset-repliesPerPage, offset)
	}

	if count == repliesPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/view/%s?%d", hash, offset+repliesPerPage), "Next page (%d-%d)", offset+repliesPerPage, offset+2*repliesPerPage)
	} else if count == repliesPerPage {
		w.Linkf(fmt.Sprintf("/users/view/%s?%d", hash, offset+repliesPerPage), "Next page (%d-%d)", offset+repliesPerPage, offset+2*repliesPerPage)
	}
}
