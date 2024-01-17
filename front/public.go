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
)

func (h *Handler) local(w text.Writer, r *request, args ...string) {
	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "url", r.URL, "error", err)
		w.Status(40, "Invalid query")
		return
	}

	if offset > h.Config.MaxOffset {
		r.Log.Warn("Offset is too big", "offset", offset)
		w.Statusf(40, "Offset must be <= %d", h.Config.MaxOffset)
		return
	}

	rows, err := r.Query(`select notes.object, persons.actor from notes left join (select object->>'inReplyTo' as id, count(*) as count from notes where inserted > unixepoch()-60*60*24*7 group by object->>'inReplyTo') replies on notes.id = replies.id join persons on notes.author = persons.id left join (select author, max(inserted) as last, count(*)/(60*60*24) as avg from notes where inserted > unixepoch()-60*60*24*7 group by author) stats on notes.author = stats.author where notes.public = 1 and notes.host = $1 order by notes.inserted / 86400 desc, replies.count desc, stats.avg asc, stats.last asc, notes.inserted desc limit $2 offset $3;`, h.Domain, h.Config.PostsPerPage, offset)
	if err != nil {
		r.Log.Warn("Failed to fetch public posts", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		noteString := ""
		var meta noteMetadata
		if err := rows.Scan(&noteString, &meta.Author); err != nil {
			r.Log.Warn("Failed to scan post", "error", err)
			continue
		}

		notes.Store(noteString, meta)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Titlef("📡 This Planet (%d-%d)", offset, offset+h.Config.PostsPerPage)
	} else {
		w.Title("📡 This Planet")
	}

	if count == 0 {
		w.Text("No posts.")
	} else {
		r.PrintNotes(w, notes, true, true, true)
	}

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Separator()
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage && offset+h.Config.PostsPerPage <= h.Config.MaxOffset {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}
}

func (h *Handler) federated(w text.Writer, r *request, args ...string) {
	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "url", r.URL, "error", err)
		w.Status(40, "Invalid query")
		return
	}

	if offset > h.Config.MaxOffset {
		r.Log.Warn("Offset is too big", "offset", offset)
		w.Statusf(40, "Offset must be <= %d", h.Config.MaxOffset)
		return
	}

	rows, err := r.Query(`select notes.object, persons.actor, groups.actor from notes join persons on notes.author = persons.id left join (select author, max(inserted) as last, count(*)/(60*60*24) as avg from notes where inserted > unixepoch()-60*60*24*7 group by author) stats on notes.author = stats.author left join (select id, actor from persons where actor->>'type' = 'Group') groups on groups.id = notes.groupid where notes.public = 1 group by notes.id order by notes.inserted / 3600 desc, stats.avg asc, stats.last asc, notes.inserted desc limit $1 offset $2;`, h.Config.PostsPerPage, offset)
	if err != nil {
		r.Log.Warn("Failed to fetch federated posts", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		noteString := ""
		var meta noteMetadata
		if err := rows.Scan(&noteString, &meta.Author, &meta.Group); err != nil {
			r.Log.Warn("Failed to scan post", "error", err)
			continue
		}

		notes.Store(noteString, meta)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Titlef("✨️ FOMO From Outer Space (%d-%d)", offset, offset+h.Config.PostsPerPage)
	} else {
		w.Title("✨️ FOMO From Outer Space")
	}

	r.PrintNotes(w, notes, true, true, true)

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Separator()
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage && offset+h.Config.PostsPerPage <= h.Config.MaxOffset {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}
}

func (h *Handler) home(w text.Writer, r *request, args ...string) {
	if r.User != nil {
		w.Redirect("/users")
		return
	}

	w.OK()
	w.Raw(logoAlt, logo)
	w.Title(h.Domain)
	w.Textf("Welcome, fedinaut! %s is an instance of tootik, a federated nanoblogging service.", h.Domain)
}
