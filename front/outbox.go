/*
Copyright 2023 - 2026 Dima Krasner

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
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
)

func writeMetadataField(field ap.Attachment, w text.Writer) {
	raw, links := getTextAndLinks(field.Val, 64, 2)

	if len(raw) > 1 {
		w.Quotef("%s: %s […]", field.Name, raw[0])
		return
	}

	if len(links) == 0 || len(links) > 1 {
		w.Quotef("%s: %s", field.Name, raw[0])
		return
	}

	for link := range links.Keys() {
		if link == raw[0] {
			w.Link(link, field.Name)
		} else {
			w.Linkf(link, "%s: %s", field.Name, raw[0])
		}
		break
	}
}

func (h *Handler) userOutbox(w text.Writer, r *Request, args ...string) {
	arg := args[1]

	var actor ap.Actor
	if err := h.DB.QueryRowContext(r.Context, `select json(actor) from persons where id = 'https://' || $1 or slug = $1`, arg).Scan(&actor); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Info("Person was not found", "actor", arg)
		w.Status(40, "User not found")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find person by ID", "actor", arg, "error", err)
		w.Error()
		return
	}

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "url", r.URL, "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.Log.Info("Viewing outbox", "actor", actor.ID, "offset", offset)

	var rows *sql.Rows
	if actor.Type == ap.Group && r.User == nil {
		// unauthenticated users can only see public posts in a group
		rows, err = h.DB.QueryContext(
			r.Context,
			`select page.slug, json(page.object), json(authors.actor), null, page.inserted, page.nreplies, page.nquotes, page.nshares, null from (
				select u.slug, u.object, u.author, max(u.inserted) as inserted, max(u.nreplies) as nreplies, max(u.nquotes) as nquotes, max(u.nshares) as nshares, max(u.pulse) as pulse from (
					select notes.slug, notes.object, notes.author, shares.inserted, notes.nreplies, notes.nquotes, notes.nshares, notes.pulse from shares
					join notes on notes.id = shares.note
					where shares.by = $1 and notes.public = 1 and notes.object->>'$.inReplyTo' is null
					union all
					select notes.slug, notes.object, notes.author, notes.inserted, notes.nreplies, notes.nquotes, notes.nshares, notes.pulse from notes
					where notes.author = $1 and notes.public = 1 and notes.object->>'$.inReplyTo' is null
				) u
				group by u.slug
				order by max(pulse) / 86400 desc, nreplies desc, pulse desc
				limit $2 offset $3
			) page
			join persons authors on authors.id = page.author
			order by page.pulse / 86400 desc, page.nreplies desc, page.pulse desc`,
			actor.ID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if actor.Type == ap.Group && r.User != nil {
		// users can see public posts in a group and non-public posts if they follow the group
		rows, err = h.DB.QueryContext(
			r.Context,
			`select page.slug, json(page.object), json(authors.actor), null, page.inserted, page.nreplies, page.nquotes, page.nshares, null from (
				select u.slug, u.object, u.author, u.inserted, max(u.nreplies) as nreplies, max(u.nquotes) as nquotes, max(u.nshares) as nshares, max(u.pulse) as pulse from (
					select notes.slug, notes.object, notes.author, shares.inserted, notes.nreplies, notes.nquotes, notes.nshares, notes.pulse from shares
					join notes on notes.id = shares.note
					where
						shares.by = $1 and
						(
							notes.public = 1 or
							exists (select 1 from follows where follower = $2 and followed = $1 and accepted = 1)
						) and
						notes.object->>'$.inReplyTo' is null
					union all
					select notes.slug, notes.object, notes.author, notes.inserted, notes.nreplies, notes.nquotes, notes.nshares, notes.pulse from notes
					where
						notes.author = $1 and
						(
							notes.public = 1 or
							exists (select 1 from follows where follower = $2 and followed = $1 and accepted = 1)
						) and
						notes.object->>'$.inReplyTo' is null
				) u
				group by u.slug
				order by max(pulse) / 86400 desc, nreplies desc, pulse desc
				limit $3 offset $4
			) page
			join persons authors on authors.id = page.author
			order by page.pulse / 86400 desc, page.nreplies desc, page.pulse desc`,
			actor.ID,
			r.User.ID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if r.User == nil {
		// unauthenticated users can only see public posts
		rows, err = h.DB.QueryContext(
			r.Context,
			`select u.slug, json(u.object), json(u.actor), json(u.sharer), max(u.inserted), u.nreplies, u.nquotes, u.nshares, json(parent_authors.actor) from (
				select notes.slug, persons.actor, notes.object, notes.inserted, null as sharer, notes.nreplies, notes.nquotes, notes.nshares from notes
				join persons on persons.id = $1
				where notes.author = $1 and notes.public = 1
				union all
				select notes.slug, authors.actor, notes.object, shares.inserted, sharers.actor as by, notes.nreplies, notes.nquotes, notes.nshares from
				shares
				join notes on notes.id = shares.note
				join persons authors on authors.id = notes.author
				join persons sharers on sharers.id = $1
				where shares.by = $1 and notes.public = 1
			) u
			left join notes parent_notes on parent_notes.id = u.object->>'$.inReplyTo'
			left join persons parent_authors on parent_authors.id = parent_notes.author
			group by u.slug
			order by max(u.inserted) desc limit $2 offset $3`,
			actor.ID,
			h.Config.PostsPerPage,
			offset,
		)
	} else if r.User.ID == actor.ID {
		// users can see all their posts
		rows, err = h.DB.QueryContext(
			r.Context,
			`select u.slug, json(u.object), json(u.actor), json(u.sharer), max(u.inserted), u.nreplies, u.nquotes, u.nshares, json(parent_authors.actor) from (
				select notes.slug, persons.actor, notes.object, notes.inserted, null as sharer, notes.nreplies, notes.nquotes, notes.nshares from notes
				join persons on persons.id = notes.author
				where notes.author = $1
				union all
				select notes.slug, authors.actor, notes.object, shares.inserted, sharers.actor as by, notes.nreplies, notes.nquotes, notes.nshares from shares
				join notes on notes.id = shares.note
				join persons authors on authors.id = notes.author
				join persons sharers on sharers.id = $1
				where shares.by = $1
			) u
			left join notes parent_notes on parent_notes.id = u.object->>'$.inReplyTo'
			left join persons parent_authors on parent_authors.id = parent_notes.author
			group by u.slug
			order by max(u.inserted) desc limit $2 offset $3`,
			actor.ID,
			h.Config.PostsPerPage,
			offset,
		)
	} else {
		// users can see only public posts by others, posts to followers if following, and DMs
		rows, err = h.DB.QueryContext(
			r.Context,
			`select page.slug, json(page.object), json(authors.actor), json(sharers.actor), page.inserted, page.nreplies, page.nquotes, page.nshares, json(parent_authors.actor) from (
				select u.slug, u.object, u.author, u.sharer_id, max(u.nreplies) as nreplies, max(u.nquotes) as nquotes, max(u.nshares) as nshares, max(u.inserted) as inserted from (
					select notes.slug, notes.author, notes.object, notes.inserted, null as sharer_id, notes.nreplies, notes.nquotes, notes.nshares from notes
					where notes.author = $1 and notes.public = 1
					union
					select notes.slug, notes.author, notes.object, notes.inserted, null as sharer_id, notes.nreplies, notes.nquotes, notes.nshares from notes
					where
						notes.author = $1 and (
							$2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = $2)) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = $2))
						)
					union
					select notes.slug, notes.author, notes.object, notes.inserted, null as sharer_id, notes.nreplies, notes.nquotes, notes.nshares from notes
					where
						notes.public = 0 and
						notes.author = $1 and
						exists (select 1 from persons where persons.id = $1 and (
							persons.actor->>'$.followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = persons.actor->>'$.followers')) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = persons.actor->>'$.followers'))
						)) and
						exists (select 1 from follows where follower = $2 and followed = $1 and accepted = 1)
					union all
					select notes.slug, notes.author, notes.object, shares.inserted, $1 as sharer_id, notes.nreplies, notes.nquotes, notes.nshares from
					shares
					join notes on notes.id = shares.note
					where shares.by = $1 and notes.public = 1
				) u
				group by u.slug
				order by max(u.inserted) desc limit $3 offset $4
			) page
			join persons authors on authors.id = page.author
			left join persons sharers on sharers.id = page.sharer_id
			left join notes parent_notes on parent_notes.id = page.object->>'$.inReplyTo'
			left join persons parent_authors on parent_authors.id = parent_notes.author
			order by page.inserted desc`,
			actor.ID,
			r.User.ID,
			h.Config.PostsPerPage,
			offset,
		)
	}
	if err != nil {
		r.Log.Warn("Failed to fetch posts", "actor", actor.ID, "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	w.OK()

	displayName := h.getActorDisplayName(&actor)

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

	if offset == 0 && len(actor.Icon) > 0 && actor.Icon[0].URL != "" {
		w.Link(actor.Icon[0].URL, "Avatar")
	} else if offset == 0 {
		w.Text("No avatar.")
	}

	if offset == 0 && actor.Image != nil && actor.Image.URL != "" {
		w.Link(actor.Image.URL, "Header")
	}

	if offset == 0 && actor.MovedTo != "" {
		w.Linkf("/users/outbox/"+idLink(actor.MovedTo), "Moved to %s", actor.MovedTo)
	}

	if offset == 0 {
		w.Empty()
		w.Subtitle("Bio")

		if len(summary) > 0 {
			for _, line := range summary {
				w.Quote(line)
			}
			for link, alt := range links.All() {
				if alt == "" {
					w.Link(link, link)
				} else {
					w.Link(link, alt)
				}
			}
		} else {
			w.Text("No bio.")
		}

		w.Empty()
		w.Subtitle("Metadata")

		if actor.Published == (ap.Time{}) {
			w.Text("Joined: ?")
		} else {
			w.Textf("Joined: %s", actor.Published.Format(time.DateOnly))
		}

		for _, prop := range actor.Attachment {
			if prop.Type != ap.PropertyValue || prop.Name == "" {
				continue
			}

			writeMetadataField(prop, w)
		}
	}

	w.Empty()
	w.Subtitle("Posts")

	count := h.PrintNotes(w, r, rows, true, actor.Type != ap.Group, "No posts.")

	if offset >= h.Config.PostsPerPage || count == h.Config.PostsPerPage {
		w.Empty()
		w.Subtitle("Navigation")
	}

	if offset >= h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.PostsPerPage), "Previous page (%d-%d)", offset-h.Config.PostsPerPage, offset)
	}

	if count == h.Config.PostsPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.PostsPerPage), "Next page (%d-%d)", offset+h.Config.PostsPerPage, offset+2*h.Config.PostsPerPage)
	}

	if r.User != nil && actor.ID != r.User.ID {
		w.Empty()
		w.Subtitle("Actions")

		var accepted sql.NullInt32
		if err := h.DB.QueryRowContext(r.Context, `select accepted from follows where follower = ? and followed = ?`, r.User.ID, actor.ID).Scan(&accepted); actor.ManuallyApprovesFollowers && errors.Is(err, sql.ErrNoRows) {
			w.Linkf("/users/follow/"+arg, "⚡ Follow %s (requires approval)", actor.PreferredUsername)
		} else if errors.Is(err, sql.ErrNoRows) {
			w.Linkf("/users/follow/"+arg, "⚡ Follow %s", actor.PreferredUsername)
		} else if err != nil {
			r.Log.Warn("Failed to check if user is followed", "actor", actor.ID, "error", err)
		} else if accepted.Valid && accepted.Int32 == 0 {
			w.Linkf("/users/unfollow/"+arg, "🔌 Unfollow %s (rejected)", actor.PreferredUsername)
		} else {
			w.Linkf("/users/unfollow/"+arg, "🔌 Unfollow %s", actor.PreferredUsername)
		}
	}
}
