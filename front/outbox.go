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
	"path/filepath"
)

func outbox(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	var actorID, actorString string
	if err := r.QueryRow(`select id, actor from persons where hash = ?`, hash).Scan(&actorID, &actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Info("Person was not found", "hash", hash)
		w.Status(40, "User not found")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find person by hash", "hash", hash, "error", err)
		w.Error()
		return
	}

	actor := ap.Actor{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		r.Log.Warn("Failed to unmarshal actor", "hash", hash, "error", err)
		w.Error()
		return
	}

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "url", r.URL, "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.AddLogContext("actor", actorID)

	r.Log.Info("Viewing outbox", "offset", offset)

	var rows *sql.Rows
	if actor.Type == ap.Group {
		// if this is a group, show posts sent to this group instead of showing posts by the group
		rows, err = r.Query(`select object, actor, null from (select notes.object, persons.actor from (select object, author, inserted from notes where public = 1 and $1 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) and object->'inReplyTo' is null) notes join persons on persons.id = notes.author order by notes.inserted desc limit $2 offset $3)`, actorID, postsPerPage, offset)
	} else {
		rows, err = r.Query(`select object, $1, g from (select notes.object, notes.inserted, groups.actor as g from (select id, object, inserted, groupid from notes where public = 1 and author = $2 union select notes.id, notes.object, notes.inserted, notes.groupid from notes join persons on persons.actor->>'followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = persons.actor->>'followers')) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = persons.actor->>'followers')) where notes.public = 0 and notes.author = $2 and persons.id = $2) notes left join (select id, actor from persons where actor->>'type' = 'Group') groups on groups.id = notes.groupid group by notes.id) order by inserted desc limit $3 offset $4`, actorString, actorID, postsPerPage, offset)
	}
	if err != nil {
		r.Log.Warn("Failed to fetch posts", "error", err)
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

	displayName := getActorDisplayName(&actor, r.Log)

	var summary []string
	var links []string
	if offset == 0 && actor.Summary != "" {
		summary, links = getTextAndLinks(actor.Summary, -1, -1)
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
	} else if actor.Type == ap.Group {
		r.PrintNotes(w, notes, true, true)
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
		var followed int
		if err := r.QueryRow(`select exists (select 1 from follows where follower = ? and followed = ?)`, r.User.ID, actorID).Scan(&followed); err != nil {
			r.Log.Warn("Failed to check if user is followed", "folowed", actorID, "error", err)
		} else if followed == 0 {
			w.Separator()
			w.Linkf(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(actorID))), "⚡ Follow %s", actor.PreferredUsername)
		} else {
			w.Separator()
			w.Linkf(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(actorID))), "🔌 Unfollow %s", actor.PreferredUsername)
		}

		var following int
		if err := r.QueryRow(`select exists (select 1 from follows where follower = ? and followed = ? and accepted = 1)`, actorID, r.User.ID).Scan(&following); err != nil {
			r.Log.Warn("Failed to check if user is a follower", "follower", actorID, "error", err)
		} else if following == 1 {
			w.Linkf(fmt.Sprintf("/users/dm/%x", sha256.Sum256([]byte(actorID))), "📟 Message %s", actor.PreferredUsername)
		}
	}
}
