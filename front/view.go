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
	"net/url"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/dbx"
	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) view(w text.Writer, r *Request, args ...string) {
	postID := "https://" + args[1]

	offset, err := getOffset(r.URL)
	if err != nil {
		r.Log.Info("Failed to parse query", "error", err)
		w.Status(40, "Invalid query")
		return
	}

	r.Log.Info("Viewing post", "post", postID)

	var note ap.Object
	var author ap.Actor
	var group sql.Null[ap.Actor]

	if r.User == nil {
		err = h.DB.QueryRowContext(
			r.Context,
			`
			select json(notes.object), json(persons.actor), json(groups.actor) from notes
			join persons on persons.id = notes.author
			left join (select id, actor from persons where actor->>'$.type' = 'Group') groups on exists (select 1 from shares where shares.by = groups.id and shares.note = $1)
			where
				notes.id = $1 and
				notes.public = 1
			`,
			postID,
		).Scan(&note, &author, &group)
	} else {
		err = h.DB.QueryRowContext(
			r.Context,
			`
			select json(notes.object), json(persons.actor), json(groups.actor) from notes
			join persons on persons.id = notes.author
			left join (select id, actor from persons where actor->>'$.type' = 'Group') groups on exists (select 1 from shares where shares.by = groups.id and shares.note = $1)
			where
				notes.id = $1 and
				(
					notes.public = 1 or
					notes.author = $2 or
					$2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = $2)) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = $2)) or
					exists (
						select 1 from (
							select persons.id, persons.actor->>'$.followers' as followers, persons.actor->>'$.type' as type from persons
							join follows on follows.followed = persons.id
							where
								follows.follower = $2 and
								follows.accepted = 1
						) follows
						where
							follows.followers in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
							(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = follows.followers)) or
							(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = follows.followers)) or
							(follows.type = 'Group' and exists (select 1 from shares where shares.by = follows.id and shares.note = notes.id))
					)
				)
			`,
			postID,
			r.User.ID,
		).Scan(&note, &author, &group)
	}
	if err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Info("Post was not found", "post", postID)
		w.Status(40, "Post not found")
		return
	} else if err != nil {
		r.Log.Info("Failed to find post", "post", postID, "error", err)
		w.Error()
		return
	}

	w.OK()

	if offset > 0 {
		w.Titlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+h.Config.RepliesPerPage)
	} else {
		if note.InReplyTo != "" {
			w.Titlef("💬 Reply by %s", author.PreferredUsername)

			w.Subtitle("Context")

			if rows, err := dbx.QueryCollect[struct {
				Note   ap.Object
				Author ap.Actor
				Depth  int
			}](
				r.Context,
				h.DB,
				`
				select json(note), json(author), max_depth from
				(
					with recursive thread(id, note, author, depth) as (
						select notes.id, notes.object as note, persons.actor as author, 1 as depth
						from notes
						join persons on persons.id = notes.author
						where notes.id = ?
						union all
						select notes.id, notes.object as note, persons.actor as author, 0 as depth
						from notes
						join persons on persons.id = notes.author
						where notes.object->>'$.context' = ? and notes.object->>'$.inReplyTo' is null
						union all
						select notes.id, notes.object as note, persons.actor as author, t.depth + 1
						from thread t
						join notes on notes.id = t.note->>'$.inReplyTo'
						join persons on persons.id = notes.author
					)
					select note, author, max(depth) as max_depth from thread group by id order by note->'$.inReplyTo' is null desc, max_depth limit ?
				)
				order by max_depth desc
				`,
				note.InReplyTo,
				note.BackfillContext,
				h.Config.PostContextDepth,
			); err != nil {
				r.Log.Info("Failed to fetch context", "error", err)
			} else if len(rows) == 0 {
				w.Text("No context.")
			} else {
				for i := range rows {
					if i == 0 && rows[0].Note.InReplyTo != "" {
						// show a marker if the thread head is a reply (i.e. we don't have the actual head)
						w.Text("[…]")
						w.Empty()
					} else if i == 1 && rows[0].Note.InReplyTo == "" && rows[0].Depth > 0 && rows[0].Depth-rows[i].Depth == 2 {
						// show the number of hidden replies if we only display the head and the bottom replies
						w.Empty()

						if r.User == nil {
							w.Link("/view/"+strings.TrimPrefix(rows[i].Note.InReplyTo, "https://"), "[1 reply]")
						} else {
							w.Link("/users/view/"+strings.TrimPrefix(rows[i].Note.InReplyTo, "https://"), "[1 reply]")
						}

						w.Empty()
					} else if i == 1 && rows[0].Note.InReplyTo == "" && rows[0].Depth > 0 && rows[i].Depth < rows[0].Depth-1 {
						w.Empty()

						if r.User == nil {
							w.Linkf("/view/"+strings.TrimPrefix(rows[i].Note.InReplyTo, "https://"), "[%d replies]", rows[0].Depth-rows[i].Depth-1)
						} else {
							w.Linkf("/users/view/"+strings.TrimPrefix(rows[i].Note.InReplyTo, "https://"), "[%d replies]", rows[0].Depth-rows[i].Depth-1)
						}

						w.Empty()
					} else if i > 0 {
						// put an empty line between replies
						w.Empty()
					}

					if r.User == nil {
						w.Linkf("/view/"+strings.TrimPrefix(rows[i].Note.ID, "https://"), "%s %s", rows[i].Note.Published.Time.Format(time.DateOnly), rows[i].Author.PreferredUsername)
					} else {
						w.Linkf("/users/view/"+strings.TrimPrefix(rows[i].Note.ID, "https://"), "%s %s", rows[i].Note.Published.Time.Format(time.DateOnly), rows[i].Author.PreferredUsername)
					}

					contentLines, _ := h.getCompactNoteContent(&rows[i].Note)
					for _, line := range contentLines {
						w.Quote(line)
					}

					if len(rows) == 1 && rows[0].Note.InReplyTo == "" && rows[0].Depth > 2 {
						w.Empty()

						if r.User == nil {
							w.Linkf("/view/"+strings.TrimPrefix(note.InReplyTo, "https://"), "[%d replies]", rows[0].Depth-1)
						} else {
							w.Linkf("/users/view/"+strings.TrimPrefix(note.InReplyTo, "https://"), "[%d replies]", rows[0].Depth-1)
						}
					} else if len(rows) == 1 && rows[0].Note.InReplyTo == "" && rows[0].Depth == 2 {
						w.Empty()

						if r.User == nil {
							w.Link("/view/"+strings.TrimPrefix(note.InReplyTo, "https://"), "[1 reply]")
						} else {
							w.Link("/users/view/"+strings.TrimPrefix(note.InReplyTo, "https://"), "[1 reply]")
						}
					} else if rows[i].Depth == 0 {
						w.Empty()
						w.Text("[…]")
					}
				}
			}

			w.Empty()
			w.Subtitle("Reply")
		} else if note.IsPublic() {
			w.Titlef("📣 Post by %s", author.PreferredUsername)
		} else {
			w.Titlef("🔔 Post by %s", author.PreferredUsername)
		}

		contentLines, links, hashtags, mentionedUsers := h.getNoteContent(&note, false)

		for mentionID := range mentionedUsers.Keys() {
			var mentionUserName string
			if err := h.DB.QueryRowContext(r.Context, `select actor->>'$.preferredUsername' from persons where id = ?`, mentionID).Scan(&mentionUserName); err != nil && errors.Is(err, sql.ErrNoRows) {
				r.Log.Warn("Mentioned user is unknown", "mention", mentionID)
				continue
			} else if err != nil {
				r.Log.Warn("Failed to get mentioned user name", "mention", mentionID, "error", err)
				continue
			}

			if r.User == nil {
				links.Store("/outbox/"+strings.TrimPrefix(mentionID, "https://"), mentionUserName)
			} else {
				links.Store("/users/outbox/"+strings.TrimPrefix(mentionID, "https://"), mentionUserName)
			}
		}

		if r.User == nil && group.Valid {
			links.Store("/outbox/"+strings.TrimPrefix(group.V.ID, "https://"), "🔄 "+group.V.PreferredUsername)
		} else if group.Valid {
			links.Store("/users/outbox/"+strings.TrimPrefix(group.V.ID, "https://"), "🔄️ "+group.V.PreferredUsername)
		} else if note.IsPublic() {
			var rows *sql.Rows
			var err error
			if r.User == nil {
				rows, err = h.DB.QueryContext(
					r.Context,
					`select id, username from
					(
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 1 as rank from shares
						join notes on notes.id = shares.note
						join persons on persons.id = shares.by
						where shares.note = $1 and persons.actor->>'$.type' = 'Group'
						union all
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 2 as rank from shares
						join notes on notes.id = shares.note
						join persons on persons.id = shares.by
						where shares.note = $1
						union all
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 3 as rank from shares
						join persons on persons.id = shares.by
						where shares.note = $1 and persons.host = $2
						union all
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 4 as rank from shares
						join persons on persons.id = shares.by
						where shares.note = $1 and persons.host != $2
					)
					group by id
					order by min(rank), inserted limit $3`,
					note.ID,
					h.Domain,
					h.Config.SharesPerPost,
				)
			} else {
				rows, err = h.DB.QueryContext(
					r.Context,
					`select id, username from
					(
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 1 as rank from shares
						join notes on notes.id = shares.note
						join persons on persons.id = shares.by
						where shares.note = $1 and persons.actor->>'$.type' = 'Group'
						union all
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 2 as rank from shares
						join notes on notes.id = shares.note
						join persons on persons.id = shares.by
						where shares.note = $1
						union all
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 3 as rank from shares
						join follows on follows.followed = shares.by
						join persons on persons.id = follows.followed
						where shares.note = $1 and follows.follower = $2 and follows.accepted = 1
						union all
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 4 as rank from shares
						join persons on persons.id = shares.by
						where shares.note = $1 and persons.host = $3
						union all
						select persons.id, persons.actor->>'$.preferredUsername' as username, shares.inserted, 5 as rank from shares
						join persons on persons.id = shares.by
						where shares.note = $1 and persons.host != $3
					)
					group by id
					order by min(rank), inserted limit $4`,
					note.ID,
					r.User.ID,
					h.Domain,
					h.Config.SharesPerPost,
				)
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				r.Log.Warn("Failed to query sharers", "error", err)
			} else if err == nil {
				if rows, err := dbx.CollectRows[struct {
					SharerID, SharerName string
				}](
					rows,
					1,
					func(err error) bool {
						r.Log.Warn("Failed to scan sharer", "error", err)
						return true
					},
				); err != nil {
					r.Log.Warn("Failed to query sharers", "error", err)
				} else {
					for _, row := range rows {
						links.Store("/users/outbox/"+strings.TrimPrefix(row.SharerID, "https://"), "🔄 "+row.SharerName)
					}
				}
				rows.Close()
			}
		}

		if quotes, err := dbx.QueryCollectIgnore[struct {
			QuoteID, Quoter string
		}](
			r.Context,
			h.DB,
			func(err error) bool {
				r.Log.Warn("Failed to scan quoter", "error", err)
				return true
			},
			`
			select notes.id, persons.actor->>'$.preferredUsername' from
			notes join persons on persons.id = notes.author
			where notes.object->>'$.quote' = ?
			order by notes.inserted desc
			limit ?
			`,
			note.ID,
			h.Config.QuotesPerPost,
		); err != nil {
			r.Log.Warn("Failed to query quotes", "error", err)
		} else {
			for _, row := range quotes {
				if r.User == nil {
					links.Store("/view/"+strings.TrimPrefix(row.QuoteID, "https://"), "♻️ "+row.Quoter)
				} else {
					links.Store("/users/view/"+strings.TrimPrefix(row.QuoteID, "https://"), "♻️ "+row.Quoter)
				}
			}
		}

		title := note.Published.Format(time.DateOnly)
		if note.Updated != (ap.Time{}) {
			title += " ┃ edited"
		}

		prefix := fmt.Sprintf("https://%s/", h.Domain)
		if strings.HasPrefix(note.ID, prefix) {
			w.Text(title)
		} else {
			w.Link(note.ID, title)
		}

		for _, line := range contentLines {
			w.Quote(line)
		}

		if r.User == nil {
			w.Link("/outbox/"+strings.TrimPrefix(author.ID, "https://"), author.PreferredUsername)
		} else {
			w.Link("/users/outbox/"+strings.TrimPrefix(author.ID, "https://"), author.PreferredUsername)
		}

		for link, alt := range links.All() {
			if alt == "" {
				w.Link(link, link)
			} else {
				if first, _, ok := strings.Cut(alt, "\n"); ok {
					w.Link(link, first+"[…]")
				} else {
					w.Link(link, alt)
				}
			}
		}

		for tag := range hashtags.Values() {
			var exists int
			if err := h.DB.QueryRowContext(r.Context, `select exists (select 1 from hashtags where hashtag = ? and note != ?)`, tag, note.ID).Scan(&exists); err != nil {
				r.Log.Warn("Failed to check if hashtag is used by other posts", "note", note.ID, "hashtag", tag)
				continue
			}

			if exists == 1 && r.User == nil {
				w.Linkf("/hashtag/"+tag, "Posts tagged #%s", tag)
			} else if exists == 1 {
				w.Linkf("/users/hashtag/"+tag, "Posts tagged #%s", tag)
			}
		}

		if r.User != nil && ap.Canonical(note.AttributedTo) == ap.Canonical(r.User.ID) && note.Type != ap.Question && note.Name == "" { // polls and votes cannot be edited
			w.Link("/users/edit/"+strings.TrimPrefix(note.ID, "https://"), "🩹 Edit")
			w.Link(fmt.Sprintf("titan://%s/users/upload/edit/%s", h.Domain, strings.TrimPrefix(note.ID, "https://")), "Upload edited post")
		}
		if r.User != nil && ap.Canonical(note.AttributedTo) == ap.Canonical(r.User.ID) {
			w.Link("/users/delete/"+strings.TrimPrefix(note.ID, "https://"), "💣 Delete")
		}
		if r.User != nil && note.Type == ap.Question && note.Closed == (ap.Time{}) && (note.EndTime == (ap.Time{}) || time.Now().Before(note.EndTime.Time)) {
			options := note.OneOf
			if len(options) == 0 {
				options = note.AnyOf
			}
			for _, option := range options {
				w.Linkf(fmt.Sprintf("/users/reply/%s?%s", strings.TrimPrefix(note.ID, "https://"), url.PathEscape(option.Name)), "📮 Vote %s", option.Name)
			}
		}

		if r.User != nil && note.IsPublic() && ap.Canonical(note.AttributedTo) != ap.Canonical(r.User.ID) {
			var shared int
			if err := h.DB.QueryRowContext(r.Context, `select exists (select 1 from shares where note = ? and by = ?)`, note.ID, r.User.ID).Scan(&shared); err != nil {
				r.Log.Warn("Failed to check if post is shared", "id", note.ID, "error", err)
			} else if shared == 0 {
				w.Link("/users/share/"+strings.TrimPrefix(note.ID, "https://"), "🔁 Share")
			} else {
				w.Link("/users/unshare/"+strings.TrimPrefix(note.ID, "https://"), "🔄️ Unshare")
			}
		}

		if r.User != nil {
			var bookmarked int
			if err := h.DB.QueryRowContext(r.Context, `select exists (select 1 from bookmarks where note = ? and by = ?)`, note.ID, r.User.ID).Scan(&bookmarked); err != nil {
				r.Log.Warn("Failed to check if post is bookmarked", "id", note.ID, "error", err)
			} else if bookmarked == 0 {
				w.Link("/users/bookmark/"+strings.TrimPrefix(note.ID, "https://"), "🔖 Bookmark")
			} else {
				w.Link("/users/unbookmark/"+strings.TrimPrefix(note.ID, "https://"), "🔖 Unbookmark")
			}
		}

		if r.User != nil {
			if note.CanQuote() {
				w.Link("/users/quote/"+strings.TrimPrefix(note.ID, "https://"), "♻️ Quote")
				w.Link(fmt.Sprintf("titan://%s/users/upload/quote/%s", h.Domain, strings.TrimPrefix(note.ID, "https://")), "Upload quote")
			}

			w.Link("/users/reply/"+strings.TrimPrefix(note.ID, "https://"), "💬 Reply")
			w.Link(fmt.Sprintf("titan://%s/users/upload/reply/%s", h.Domain, strings.TrimPrefix(note.ID, "https://")), "Upload reply")
		}

		if note.Type == ap.Question && offset == 0 {
			options := note.OneOf
			if len(options) == 0 {
				options = note.AnyOf
			}

			if len(options) > 0 {
				w.Empty()

				if note.VotersCount == 1 {
					w.Subtitle("📊 Results (one voter)")
				} else {
					w.Subtitlef("📊 Results (%d voters)", note.VotersCount)
				}

				labels := make([]string, 0, len(options))
				votes := make([]int64, 0, len(options))

				for _, option := range options {
					labels = append(labels, option.Name)
					votes = append(votes, option.Replies.TotalItems)
				}

				w.Raw("Results graph", graph.Bars(labels, votes))
			}
		}

		if note.Quote != "" {
			w.Empty()
			w.Subtitle("Quote")

			var quote ap.Object
			var quoteAuthor string
			if err := h.DB.QueryRowContext(
				r.Context,
				`
				select json(notes.object), persons.actor->>'$.preferredUsername' from notes
				join persons on persons.id = notes.author
				where notes.id = ?
				`,
				note.Quote,
			).Scan(&quote, &quoteAuthor); errors.Is(err, sql.ErrNoRows) {
				w.Text("[Missing]")
			} else if err != nil {
				r.Log.Warn("Failed to scan quote", "error", err)
				w.Text("[Error]")
			} else {
				if r.User == nil {
					w.Linkf("/view/"+strings.TrimPrefix(quote.ID, "https://"), "%s %s", quote.Published.Time.Format(time.DateOnly), quoteAuthor)
				} else {
					w.Linkf("/users/view/"+strings.TrimPrefix(quote.ID, "https://"), "%s %s", quote.Published.Time.Format(time.DateOnly), quoteAuthor)
				}

				quoteLines, _ := h.getCompactNoteContent(&quote)
				for _, line := range quoteLines {
					w.Quote(line)
				}
			}
		}

		if offset > 0 {
			w.Empty()
			w.Subtitlef("💬 Replies to %s (%d-%d)", author.PreferredUsername, offset, offset+h.Config.RepliesPerPage)
		} else {
			w.Empty()
			w.Subtitlef("💬 Replies to %s", author.PreferredUsername)
		}
	}

	var replies *sql.Rows
	var count int
	if r.User == nil {
		replies, err = h.DB.QueryContext(
			r.Context,
			`
			select json(replies.object), json(persons.actor), null as sharer, replies.inserted from notes join notes replies on replies.object->>'$.inReplyTo' = notes.id
			left join persons on persons.id = replies.author
			where
				notes.id = $1 and
				replies.public = 1
			order by replies.inserted desc limit $2 offset $3
			`,
			postID,
			h.Config.RepliesPerPage,
			offset,
		)
	} else {
		replies, err = h.DB.QueryContext(
			r.Context,
			`
			select json(replies.object), json(persons.actor), null as sharer, replies.inserted from
			notes join notes replies on replies.object->>'$.inReplyTo' = notes.id
			left join persons on persons.id = replies.author
			where
				notes.id = $1 and
				(
					replies.public = 1 or
					replies.author = $2 or
					$2 in (replies.cc0, replies.to0, replies.cc1, replies.to1, replies.cc2, replies.to2) or
					(replies.to2 is not null and exists (select 1 from json_each(replies.object->'$.to') where value = $2)) or
					(replies.cc2 is not null and exists (select 1 from json_each(replies.object->'$.cc') where value = $2)) or
					exists (
						select 1 from persons
						join follows on follows.followed = persons.id
						where
							follows.follower = $2 and
							follows.accepted = 1 and
							(
								persons.actor->>'$.followers' in (replies.cc0, replies.to0, replies.cc1, replies.to1, replies.cc2, replies.to2) or
								(notes.to2 is not null and exists (select 1 from json_each(replies.object->'$.to') where value = persons.actor->>'$.followers')) or
								(notes.cc2 is not null and exists (select 1 from json_each(replies.object->'$.cc') where value = persons.actor->>'$.followers')) or
								(persons.actor->>'$.type' = 'Group' and exists (select 1 from shares where shares.by = persons.id and shares.note = replies.id))
							)
					)
				)
			order by replies.inserted desc limit $3 offset $4
			`,
			postID,
			r.User.ID,
			h.Config.RepliesPerPage,
			offset,
		)
	}
	if err != nil {
		r.Log.Warn("Failed to fetch replies", "error", err)
	} else {
		count = h.PrintNotes(w, r, replies, false, false, "No replies.")
		replies.Close()
	}

	if offset > h.Config.RepliesPerPage || offset >= h.Config.RepliesPerPage || count == h.Config.RepliesPerPage {
		w.Empty()
		w.Subtitle("Navigation")
	}

	if offset > h.Config.RepliesPerPage {
		w.Link(r.URL.Path, "First page")
	}

	if offset >= h.Config.RepliesPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset-h.Config.RepliesPerPage), "Previous page (%d-%d)", offset-h.Config.RepliesPerPage, offset)
	}

	if count == h.Config.RepliesPerPage {
		w.Linkf(fmt.Sprintf("%s?%d", r.URL.Path, offset+h.Config.RepliesPerPage), "Next page (%d-%d)", offset+h.Config.RepliesPerPage, offset+2*h.Config.RepliesPerPage)
	}
}
