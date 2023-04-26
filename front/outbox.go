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
	"github.com/dimkr/tootik/text"
	log "github.com/sirupsen/logrus"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/outbox/[0-9a-f]{64}$`)] = withUserMenu(outbox)
	handlers[regexp.MustCompile(`^/outbox/[0-9a-f]{64}$`)] = withUserMenu(outbox)
}

func outbox(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	var actorID, actorString string
	if err := r.QueryRow(`select id, actor from persons where hash = ?`, hash).Scan(&actorID, &actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.WithField("hash", hash).Info("Person was not found")
		w.Status(40, "User not found")
		return
	} else if err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to find person by hash")
		w.Error()
		return
	}

	actor := ap.Actor{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to unmarshal actor")
		w.Error()
		return
	}

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.WithField("url", r.URL.String()).WithError(err).Info("Failed to parse query")
		w.Status(40, "Invalid query")
		return
	}

	r.Log.WithFields(log.Fields{"actor": actorID, "offset": offset}).Info("Viewing outbox")

	rows, err := r.Query(`select object from (select object, inserted from notes where public = 1 and author = $1 union select notes.object, notes.inserted from notes join persons on persons.actor->>'followers' in (notes.to0, notes.to1, notes.to2, notes.cc0, notes.cc1, notes.cc2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = persons.actor->>'followers')) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = persons.actor->>'followers')) where notes.public = 0 and notes.author = $1 and persons.id = $1 order by inserted desc limit $2 offset $3)`, actorID, postsPerPage, offset)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to fetch posts")
		w.Error()
		return
	}
	defer rows.Close()

	notes := data.OrderedMap[string, sql.NullString]{}

	for rows.Next() {
		noteString := ""
		if err := rows.Scan(&noteString); err != nil {
			r.Log.WithError(err).Warn("Failed to scan post")
			continue
		}

		notes.Store(noteString, sql.NullString{})
	}
	rows.Close()

	count := len(notes)

	w.OK()

	displayName := getActorDisplayName(&actor)

	var summary []string
	var links []string
	if offset == 0 && actor.Summary != "" {
		_, summary, links = getTextAndLinks(actor.Summary, -1, -1)
	}

	if offset >= postsPerPage || count == postsPerPage {
		w.Titlef("%s (%d-%d)", displayName, offset, offset+postsPerPage)
	} else {
		w.Title(displayName)
	}

	if len(summary) > 0 {
		for _, line := range summary {
			w.Quote(line)
		}
		for _, link := range links {
			w.Link(link, link)
		}
		w.Separator()
	}

	if count == 0 {
		w.Text("No posts.")
	} else {
		r.PrintNotes(w, notes, false, true)
	}

	if offset >= postsPerPage || count == postsPerPage {
		w.Separator()
	}

	if offset >= postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/outbox/%s?%d", hash, offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	} else if offset >= postsPerPage {
		w.Linkf(fmt.Sprintf("/users/outbox/%s?%d", hash, offset-postsPerPage), "Previous page (%d-%d)", offset-postsPerPage, offset)
	}

	if count == postsPerPage && r.User == nil {
		w.Linkf(fmt.Sprintf("/outbox/%s?%d", hash, offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	} else if count == postsPerPage {
		w.Linkf(fmt.Sprintf("/users/outbox/%s?%d", hash, offset+postsPerPage), "Next page (%d-%d)", offset+postsPerPage, offset+2*postsPerPage)
	}

	if r.User != nil && actorID != r.User.ID {
		var followID string
		err := r.QueryRow(`select id from follows where follower = ? and followed = ?`, r.User.ID, actorID).Scan(&followID)
		if err != nil && errors.Is(err, sql.ErrNoRows) {
			w.Separator()
			w.Linkf(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(actorID))), "⚡ Follow %s", displayName)
		} else if err != nil {
			r.Log.WithField("followed", actorID).WithError(err).Warn("Failed to check if user is followed")
		} else {
			w.Separator()
			w.Linkf(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(actorID))), "🔌 Unfollow %s", displayName)
		}

		w.Linkf(fmt.Sprintf("/users/dm/%x", sha256.Sum256([]byte(actorID))), "📟 Message %s", displayName)
	}
}
