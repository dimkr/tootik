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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
	"strings"
)

func (h *Handler) userOutbox(w text.Writer, r *request, args ...string) {
	actorID := "https://" + args[1]

	var actorString string
	if err := r.QueryRow(`select actor from persons where id = ?`, actorID).Scan(&actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Info("Person was not found", "actor", actorID)
		w.Status(40, "User not found")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find person by ID", "actor", actorID, "error", err)
		w.Error()
		return
	}

	actor := ap.Actor{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		r.Log.Warn("Failed to unmarshal actor", "actor", actorID, "error", err)
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
	if actor.Type == ap.Group && r.User == nil {
		// unauthenticated users can only see public posts in a group
		rows, err = r.Query(
			`select notes.object, persons.actor, null from (
				select object, author from notes where object->>'audience' = $1 and public = 1
				order by notes.inserted desc limit $2 offset $3
			) notes
			join persons on persons.id = notes.author`,
			actorID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if actor.Type == ap.Group && r.User != nil {
		// users can see public posts in a group and non-public posts if they follow the group
		rows, err = r.Query(`
			select notes.object, persons.actor, null from (
				select notes.object, notes.author from notes
				where
					object->>'audience' = $1 and
					(
						public = 1 or
						exists (select 1 from follows where follower = $2 and followed = $1 and accepted = 1)
					)
					order by inserted desc limit $3 offset $4
			) notes
			join persons on persons.id = notes.author`,
			actorID,
			r.User.ID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if r.User == nil {
		// unauthenticated users can only see public posts
		rows, err = r.Query(
			`select object, persons.actor, groups.actor from (
				select id, author, object, inserted from notes
				where author = $1 and public = 1
				union
				select notes.id, notes.author, notes.object, shares.inserted from
				shares
				join notes on notes.id = shares.note
				where shares.by = $1 and notes.public = 1
			) notes
			join persons on persons.id = notes.author
			left join (
				select id, actor from persons where actor->>'type' = 'Group'
			) groups on groups.id = notes.object->>'audience'
			group by notes.id
			order by max(notes.inserted) desc limit $2 offset $3`,
			actorID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if r.User.ID == actorID {
		// users can see all their posts
		rows, err = r.Query(
			`select object, $1, g from (
				select notes.id, notes.object, groups.actor as g from (
					select id, object, inserted from notes
					where author = $2
					union
					select notes.id, notes.object, shares.inserted from
					shares
					join notes on notes.id = shares.note
					where shares.by = $2
				) notes
				left join (
					select id, actor from persons where actor->>'type' = 'Group'
				) groups on groups.id = notes.object->>'audience'
				group by notes.id
				order by max(notes.inserted) desc limit $3 offset $4
			)`,
			actorString,
			actorID,
			h.Config.PostsPerPage,
			offset,
		)
	} else {
		// users can see only public posts by others, posts to followers if following, and DMs
		rows, err = r.Query(
			`select u.object, persons.actor, groups.actor as g from (
				select id, $1 as author, object, inserted from notes
				where public = 1 and author = $1
				union
				select id, $1 as author, object, inserted from notes
				where (
					author = $1 and (
						$2 in (cc0, to0, cc1, to1, cc2, to2) or
						(to2 is not null and exists (select 1 from json_each(object->'to') where value = $2)) or
						(cc2 is not null and exists (select 1 from json_each(object->'cc') where value = $2))
					)
				)
				union
				select notes.id, $1 as author, object, notes.inserted from notes
				join persons on
					persons.actor->>'followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = persons.actor->>'followers')) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = persons.actor->>'followers'))
				where notes.public = 0 and
					notes.author = $1 and
					persons.id = $1 and
					exists (select 1 from follows where follower = $2 and followed = $1 and accepted = 1)
				union
				select notes.id, notes.author, notes.object, shares.inserted from
				shares
				join notes on notes.id = shares.note
				where shares.by = $1
			) u
			join persons on persons.id = u.author
			left join (
				select id, actor from persons where actor->>'type' = 'Group'
			) groups on groups.id = u.object->>'audience'
			group by u.id
			order by max(u.inserted) desc limit $3 offset $4`,
			actorID,
			r.User.ID,
			h.Config.PostsPerPage,
			offset,
		)
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

	displayName := h.getActorDisplayName(&actor, r.Log)

	var summary []string
	var links data.OrderedMap[string, string]
	if offset == 0 && actor.Summary != "" {
		summary, links = getTextAndLinks(actor.Summary, -1, -1)
	}

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Titlef("%s (%d-%d)", displayName, offset, offset+h.Config.PostsPerPage)
	} else {
		w.Title(displayName)
	}

	if len(summary) > 0 {
		for _, line := range summary {
			w.Quote(line)
		}
		links.Range(func(link, alt string) bool {
			if alt == "" {
				w.Link(link, link)
			} else {
				w.Linkf(link, "%s [%s]", link, alt)
			}
			return true
		})
		w.Separator()
	}

	if count == 0 {
		w.Text("No posts.")
	} else {
		r.PrintNotes(w, notes, true, true)
	}

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Separator()
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}

	if r.User != nil && actorID != r.User.ID {
		var followed int
		if err := r.QueryRow(`select exists (select 1 from follows where follower = ? and followed = ?)`, r.User.ID, actorID).Scan(&followed); err != nil {
			r.Log.Warn("Failed to check if user is followed", "folowed", actorID, "error", err)
		} else if followed == 0 {
			w.Separator()
			w.Linkf("/users/follow/"+strings.TrimPrefix(actorID, "https://"), "⚡ Follow %s", actor.PreferredUsername)
		} else {
			w.Separator()
			w.Linkf("/users/unfollow/"+strings.TrimPrefix(actorID, "https://"), "🔌 Unfollow %s", actor.PreferredUsername)
		}

		var following int
		if err := r.QueryRow(`select exists (select 1 from follows where follower = ? and followed = ? and accepted = 1)`, actorID, r.User.ID).Scan(&following); err != nil {
			r.Log.Warn("Failed to check if user is a follower", "follower", actorID, "error", err)
		} else if following == 1 {
			w.Linkf("/users/dm/"+strings.TrimPrefix(actorID, "https://"), "📟 Message %s", actor.PreferredUsername)
		}
	}
}
