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
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/text"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/local$`)] = withCache(withUserMenu(local), time.Minute*15)
	handlers[regexp.MustCompile(`^/users/local$`)] = withCache(withUserMenu(local), time.Minute*15)

	handlers[regexp.MustCompile(`^/federated$`)] = withCache(withUserMenu(federated), time.Minute*10)
	handlers[regexp.MustCompile(`^/users/federated$`)] = withCache(withUserMenu(federated), time.Minute*10)

	handlers[regexp.MustCompile(`^/$`)] = withUserMenu(home)
}

func local(w text.Writer, r *request) {
	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.WithField("url", r.URL.String()).WithError(err).Info("Failed to parse query")
		w.Status(40, "Invalid query")
		return
	}

	now := time.Now()
	since := now.Add(-time.Hour * 24 * 7)

	rows, err := r.Query(`select notes.object, persons.actor from notes left join (select object->>'inReplyTo' as id, count(*) as count from notes where inserted >= $2 group by object->>'inReplyTo') replies on notes.id = replies.id join persons on notes.author = persons.id left join (select author, max(inserted) as last, count(*) / $1 as avg from notes where inserted >= $2 group by author) stats on notes.author = stats.author left join (select followed as id, count(*) as count from follows group by followed) followers on notes.author = followers.id where notes.public = 1 and notes.author like $3 order by notes.inserted / 86400 desc, replies.count desc, followers.count desc, stats.avg asc, stats.last asc, notes.inserted / 3600 desc, notes.inserted desc limit $4 offset $5;`, now.Sub(since)/time.Hour, since.Unix(), fmt.Sprintf("https://%s/%%", cfg.Domain), postsPerPage, offset)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to fetch public posts")
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, sql.NullString]{}

	for rows.Next() {
		noteString := ""
		var actorString sql.NullString
		if err := rows.Scan(&noteString, &actorString); err != nil {
			r.Log.WithError(err).Warn("Failed to scan post")
			continue
		}

		notes.Store(noteString, actorString)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= postsPerPage || count == postsPerPage {
		w.Titlef("📡 This Planet (%d-%d)", offset, offset+postsPerPage)
	} else {
		w.Title("📡 This Planet")
	}

	if count == 0 {
		w.Text("No posts.")
	} else {
		r.PrintNotes(w, notes, true, true)
	}

	if offset >= postsPerPage || count == postsPerPage {
		w.Separator()
	}

	if offset >= postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/local?%d", offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		w.Linkf(fmt.Sprintf("/users/local?%d", offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	}

	if count == postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/local?%d", offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	} else if count == postsPerPage {
		w.Linkf(fmt.Sprintf("/users/local?%d", offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	}
}

func federated(w text.Writer, r *request) {
	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.WithField("url", r.URL.String()).WithError(err).Info("Failed to parse query")
		w.Status(40, "Invalid query")
		return
	}

	now := time.Now()
	since := time.Now().Add(-time.Hour * 24 * 7)

	rows, err := r.Query(`select notes.object, persons.actor from notes left join (select object->>'inReplyTo' as id, count(*) as count from notes where inserted >= $2 group by object->>'inReplyTo') replies on notes.id = replies.id join persons on notes.author = persons.id left join (select author, max(inserted) as last, count(*) / $1 as avg from notes where inserted >= $2 group by author) stats on notes.author = stats.author left join (select followed as id, count(*) as count from follows group by followed) followers on notes.author = followers.id where notes.public = 1 and persons.actor->>'type' = 'Person' order by notes.inserted / 86400 desc, replies.count desc, followers.count desc, stats.avg asc, stats.last asc, notes.inserted / 3600 desc, notes.inserted desc limit $3 offset $4;`, now.Sub(since)/time.Hour, since.Unix(), postsPerPage, offset)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to fetch federated posts")
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, sql.NullString]{}

	for rows.Next() {
		noteString := ""
		var actorString sql.NullString
		if err := rows.Scan(&noteString, &actorString); err != nil {
			r.Log.WithError(err).Warn("Failed to scan post")
			continue
		}

		notes.Store(noteString, actorString)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= postsPerPage || count == postsPerPage {
		w.Titlef("✨️ Outer Space (%d-%d)", offset, offset+postsPerPage)
	} else {
		w.Title("✨️ Outer Space")
	}

	r.PrintNotes(w, notes, true, true)

	if offset >= postsPerPage || count == postsPerPage {
		w.Separator()
	}

	if offset >= postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/federated?%d", offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		w.Linkf(fmt.Sprintf("/users/federated?%d", offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	}

	if count == postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/federated?%d", offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	} else if count == postsPerPage {
		w.Linkf(fmt.Sprintf("/users/federated?%d", offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	}
}

func home(w text.Writer, r *request) {
	if r.User != nil {
		w.Redirect("/users")
		return
	}

	w.OK()
	w.Raw(logoAlt, logo)
	w.Empty()
	w.Title(cfg.Domain)
	w.Textf("Welcome, fedinaut! %s is a federated nanoblogging service.", cfg.Domain)
}
