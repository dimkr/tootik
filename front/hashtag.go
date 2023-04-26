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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/text"
	"path/filepath"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/hashtag/[a-zA-Z0-9]+$`)] = withCache(withUserMenu(hashtag), time.Minute*5)
	handlers[regexp.MustCompile(`^/users/hashtag/[a-zA-Z0-9]+$`)] = withCache(withUserMenu(hashtag), time.Minute*5)
}

func hashtag(w text.Writer, r *request) {
	offset, err := getOffset(r.URL)
	if err != nil {
		w.Status(40, "Invalid query")
		return
	}

	tag := filepath.Base(r.URL.Path)

	rows, err := r.Query(`select notes.object, persons.actor from notes join hashtags on notes.id = hashtags.note left join (select object->>'inReplyTo' as id, count(*) as count from notes where inserted >= unixepoch() - 7*24*60*60 group by object->>'inReplyTo') replies on notes.id = replies.id left join persons on notes.author = persons.id where notes.public = 1 and hashtags.hashtag = $1 order by replies.count desc, notes.inserted desc limit $2 offset $3`, tag, postsPerPage, offset)
	if err != nil {
		r.Log.WithField("hashtag", tag).WithError(err).Error("Failed to fetch notes by hashtag")
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, sql.NullString]{}

	for rows.Next() {
		noteString := ""
		var actorString sql.NullString
		if err := rows.Scan(&noteString, &actorString); err != nil {
			r.Log.WithField("hashtag", tag).WithError(err).Warn("Failed to scan post")
			continue
		}

		notes.Store(noteString, actorString)
	}
	rows.Close()

	count := len(notes)

	w.OK()

	if offset >= postsPerPage || count == postsPerPage {
		w.Titlef("Posts Tagged #%s (%d-%d)", tag, offset, offset+postsPerPage)
	} else {
		w.Titlef("Posts Tagged #%s", tag)
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
		w.Linkf(fmt.Sprintf("/hashtag/%s?%d", tag, offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		w.Linkf(fmt.Sprintf("/users/hashtag/%s?%d", tag, offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	}

	if count == postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/hashtag/%s?%d", tag, offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	} else if count == postsPerPage {
		w.Linkf(fmt.Sprintf("/users/hashtag/%s?%d", tag, offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	}
}
