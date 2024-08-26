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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"strings"
	"time"
)

func (h *Handler) userOutbox(w text.Writer, r *request, args ...string) {
	actorID := "https://" + args[1]

	var actor ap.Actor
	if err := r.QueryRow(`select actor from persons where id = ?`, actorID).Scan(&actor); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Info("Person was not found", "actor", actorID)
		w.Status(40, "User not found")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find person by ID", "actor", actorID, "error", err)
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
			`select notes.object, authors.actor, null, max(notes.inserted, coalesce(max(replies.inserted), 0)) from notes
			join persons authors on authors.id = notes.author
			left join notes replies on replies.object->>'$.inReplyTo' = notes.id
			where notes.object->>'$.audience' = $1 and notes.public = 1 and notes.object->>'$.inReplyTo' is null
			group by notes.id
			order by max(notes.inserted, coalesce(max(replies.inserted), 0)) / 86400 desc, count(replies.id) desc, notes.inserted desc limit $2 offset $3`,
			actorID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if actor.Type == ap.Group && r.User != nil {
		// users can see public posts in a group and non-public posts if they follow the group
		rows, err = r.Query(
			`select notes.object, authors.actor, null, max(notes.inserted, coalesce(max(replies.inserted), 0)) from notes
			join persons authors on authors.id = notes.author
			left join notes replies on replies.object->>'$.inReplyTo' = notes.id
			where
				notes.object->>'$.audience' = $1 and
				(
					notes.public = 1 or
					exists (select 1 from follows where follower = $2 and followed = $1 and accepted = 1)
				) and
				notes.object->>'$.inReplyTo' is null
			group by notes.id
			order by max(notes.inserted, coalesce(max(replies.inserted), 0)) / 86400 desc, count(replies.id) desc, notes.inserted desc limit $3 offset $4`,
			actorID,
			r.User.ID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if r.User == nil {
		// unauthenticated users can only see public posts
		rows, err = r.Query(
			`select object, actor, sharer, max(inserted) from (
				select notes.id, persons.actor, notes.object, notes.inserted, null as sharer from notes
				join persons on persons.id = $1
				where notes.author = $1 and notes.public = 1
				union all
				select notes.id, authors.actor, notes.object, shares.inserted, sharers.actor as by from
				shares
				join notes on notes.id = shares.note
				join persons authors on authors.id = notes.author
				join persons sharers on sharers.id = $1
				where shares.by = $1 and notes.public = 1
			)
			group by id
			order by max(inserted) desc limit $2 offset $3`,
			actorID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if r.User.ID == actorID {
		// users can see all their posts
		rows, err = r.Query(
			`select object, actor, sharer, max(inserted) from (
				select notes.id, persons.actor, notes.object, notes.inserted, null as sharer from notes
				join persons on persons.id = notes.author
				where notes.author = $1
				union all
				select notes.id, authors.actor, notes.object, shares.inserted, sharers.actor as by from shares
				join notes on notes.id = shares.note
				join persons authors on authors.id = notes.author
				join persons sharers on sharers.id = $1
				where shares.by = $1
			)
			group by id
			order by max(inserted) desc limit $2 offset $3`,
			actorID,
			h.Config.PostsPerPage,
			offset,
		)
	} else {
		// users can see only public posts by others, posts to followers if following, and DMs
		rows, err = r.Query(
			`select object, actor, sharer, max(inserted) from (
				select notes.id, persons.actor, notes.object, notes.inserted, null as sharer from notes
				join persons on persons.id = $1
				where notes.author = $1 and notes.public = 1
				union
				select notes.id, persons.actor, notes.object, notes.inserted, null as sharer from notes
				join persons on persons.id = $1
				where (
					notes.author = $1 and (
						$2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
						(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = $2)) or
						(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = $2))
					)
				)
				union
				select notes.id, authors.actor, object, notes.inserted, null as sharer from notes
				join persons on
					persons.actor->>'$.followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = persons.actor->>'$.followers')) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = persons.actor->>'$.followers'))
				join persons authors on authors.id = $1
				where notes.public = 0 and
					notes.author = $1 and
					persons.id = $1 and
					exists (select 1 from follows where follower = $2 and followed = $1 and accepted = 1)
				union all
				select notes.id, authors.actor, notes.object, shares.inserted, sharers.actor as by from
				shares
				join notes on notes.id = shares.note
				join persons authors on authors.id = notes.author
				join persons sharers on sharers.id = $1
				where shares.by = $1 and notes.public = 1
			)
			group by id
			order by max(inserted) desc limit $3 offset $4`,
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

	w.OK()

	displayName := h.getActorDisplayName(&actor, r.Log)

	var summary []string
	var links data.OrderedMap[string, string]
	if offset == 0 && actor.Summary != "" {
		summary, links = getTextAndLinks(actor.Summary, -1, -1)
	}

	if actor.Type != ap.Person && offset > 0 {
		w.Titlef("%s [%s] (%d-%d)", displayName, actor.Type, offset, offset+h.Config.PostsPerPage)
	} else if actor.Type != ap.Person {
		w.Titlef("%s [%s]", displayName, actor.Type)
	} else if offset > 0 {
		w.Titlef("%s (%d-%d)", displayName, offset, offset+h.Config.PostsPerPage)
	} else {
		w.Title(displayName)
	}

	showSeparator := false

	if offset == 0 && len(actor.Icon) > 0 && actor.Icon[0].URL != "" {
		w.Link(actor.Icon[0].URL, "Avatar")
		showSeparator = true
	}

	if offset == 0 && actor.Image != nil && actor.Image.URL != "" {
		w.Link(actor.Image.URL, "Header")
		showSeparator = true
	}

	if offset == 0 && actor.MovedTo != "" {
		w.Linkf("/users/outbox/"+strings.TrimPrefix(actor.MovedTo, "https://"), "Moved to %s", actor.MovedTo)
		showSeparator = true
	}

	if len(summary) > 0 {
		if showSeparator {
			w.Empty()
		}

		for _, line := range summary {
			w.Quote(line)
		}
		for link, alt := range links.All() {
			if alt == "" {
				w.Link(link, link)
			} else {
				w.Linkf(link, "%s [%s]", link, alt)
			}
		}
	}

	if offset == 0 {
		firstProperty := true

		if actor.Published != nil {
			if showSeparator {
				w.Empty()
			}

			w.Textf("Joined: %s", actor.Published.Format(time.DateOnly))

			firstProperty = false
			showSeparator = true
		}

		for _, prop := range actor.Attachment {
			if prop.Type != ap.PropertyValue || prop.Name == "" || prop.Value == "" {
				continue
			}

			raw, links := plain.FromHTML(prop.Value)
			if len(links) > 1 {
				continue
			}

			if len(summary) > 0 && firstProperty {
				w.Empty()
			}

			if len(links) == 0 {
				w.Textf("%s: %s", prop.Name, raw)
			} else {
				for link := range links.Keys() {
					w.Linkf(link, prop.Name)
					break
				}
			}

			firstProperty = false
			showSeparator = true
		}
	}

	if showSeparator {
		w.Separator()
	}

	count := r.PrintNotes(w, rows, true, actor.Type != ap.Group)
	rows.Close()

	if count == 0 {
		w.Text("No posts.")
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
	}
}
