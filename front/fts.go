/*
Copyright 2024 Dima Krasner

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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
	"net/url"
)

func fts(w text.Writer, r *request) {
	if r.URL.RawQuery == "" {
		w.Status(10, "Query")
		return
	}

	query, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Info("Failed to decode query", "url", r.URL, "error", err)
		w.Status(40, "Bad input")
		return
	}

	rows, err := r.Query("select notes.object, persons.actor, case when notes.groupid is null then null else (select actor from persons where persons.id = notes.groupid limit 1) end from notesfts join notes on notes.id = notesfts.id join persons on persons.id = notes.author where notesfts.content match ? order by rank, notes.inserted desc limit 30", query)
	if err != nil {
		r.Log.Warn("Failed to search for posts", "query", query, "error", err)
		w.Error()
		return
	}

	notes := data.OrderedMap[string, noteMetadata]{}

	for rows.Next() {
		noteString := ""
		var meta noteMetadata
		if err := rows.Scan(&noteString, &meta.Author, &meta.Group); err != nil {
			r.Log.Warn("Failed to scan search result", "error", err)
			continue
		}
		notes.Store(noteString, meta)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	w.Titlef("🔎 Search Results for '%s'", query)

	if count == 0 {
		w.Text("No results.")
	} else {
		r.PrintNotes(w, notes, true, true, false)
	}
}
