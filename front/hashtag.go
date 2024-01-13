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
	"fmt"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
	"path/filepath"
)

func (h *Handler) hashtag(w text.Writer, r *request) {
	offset, err := getOffset(r.URL)
	if err != nil {
		w.Status(40, "Invalid query")
		return
	}

	tag := filepath.Base(r.URL.Path)

	rows, err := r.Query(`select notes.object, persons.actor from notes join hashtags on notes.id = hashtags.note left join (select object->>'inReplyTo' as id, count(*) as count from notes where inserted >= unixepoch() - 7*24*60*60 group by object->>'inReplyTo') replies on notes.id = replies.id left join persons on notes.author = persons.id where notes.public = 1 and hashtags.hashtag = $1 order by replies.count desc, notes.inserted/(24*60*60) desc, notes.inserted desc limit $2 offset $3`, tag, h.Config.PostsPerPage, offset)
	if err != nil {
		r.Log.Error("Failed to fetch notes by hashtag", "hashtag", tag, "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		noteString := ""
		var meta noteMetadata
		if err := rows.Scan(&noteString, &meta.Author); err != nil {
			r.Log.Warn("Failed to scan post", "hashtag", tag, "error", err)
			continue
		}

		notes.Store(noteString, meta)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Titlef("Posts Tagged #%s (%d-%d)", tag, offset, offset+h.Config.PostsPerPage)
	} else {
		w.Titlef("Posts Tagged #%s", tag)
	}

	if count == 0 {
		w.Text("No posts.")
	} else {
		r.PrintNotes(w, notes, true, true, false)
	}

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Separator()
	}

	if offset >= h.Config.PostsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/hashtag/%s?%d", tag, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	} else if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("/users/hashtag/%s?%d", tag, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/hashtag/%s?%d", tag, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	} else if count == h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("/users/hashtag/%s?%d", tag, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}

	w.Separator()

	if r.User == nil {
		w.Link("/search", "🔎 Posts by hashtag")
	} else {
		w.Link("/users/search", "🔎 Posts by hashtag")
	}
}
