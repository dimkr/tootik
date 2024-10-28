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
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var verifiedRegex = regexp.MustCompile(`(\s*:[a-zA-Z0-9_]+:\s*)+`)

func getTextAndLinks(s string, maxRunes, maxLines int) ([]string, data.OrderedMap[string, string]) {
	raw, links := plain.FromHTML(s)

	if raw == "" {
		raw = "[no content]"
	}

	if maxRunes > 6 {
		if cut := text.WordWrap(raw, maxRunes-6, 1)[0]; len(cut) < len(raw) {
			raw = cut + " […]"
		}
	}

	lines := strings.Split(raw, "\n")

	if maxLines <= 0 || len(lines) <= maxLines {
		return lines, links
	}

	summary := make([]string, 0, maxLines)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			if len(summary) == maxLines-1 {
				// replace terminating blank line with […]
				if summary[len(summary)-1] == "" {
					summary[len(summary)-1] = "[…]"
				} else if summary[len(summary)-1] != "[…]" {
					summary = append(summary, "[…]")
				}
				break
			}

			summary = append(summary, line)
			continue
		}

		// replace multiple empty lines with one […] line
		if len(summary) > 0 && (len(summary) > 0 && summary[len(summary)-1] == "") {
			summary[len(summary)-1] = "[…]"
		} else if len(summary) == maxLines-1 && summary[len(summary)-1] != "[…]" {
			summary = append(summary, "[…]")
		} else if len(summary) == 0 || summary[len(summary)-1] != "[…]" {
			summary = append(summary, line)
		}

		if len(summary) == maxLines {
			break
		}
	}

	return summary, links
}

func (h *Handler) getDisplayName(id, preferredUsername, name string, t ap.ActorType) string {
	prefix := fmt.Sprintf("https://%s/user/", h.Domain)

	isLocal := strings.HasPrefix(id, prefix)

	emoji := "👽"
	if t == ap.Group {
		emoji = "👥"
	} else if t != ap.Person {
		emoji = "🤖"
	} else if isLocal {
		emoji = "😈"
	} else if strings.Contains(id, "masto") || strings.Contains(id, "mstdn") {
		emoji = "🐘"
	}

	displayName := preferredUsername
	if name != "" {
		displayName = name
	}

	for match := verifiedRegex.FindStringIndex(displayName); match != nil; match = verifiedRegex.FindStringIndex(displayName) {
		displayName = displayName[:match[0]] + displayName[match[1]:]
	}

	u, err := url.Parse(id)
	if err != nil {
		slog.Warn("Failed to parse user ID", "id", id, "error", err)
		return fmt.Sprintf("%s %s", emoji, displayName)
	}

	return fmt.Sprintf("%s %s (%s@%s)", emoji, displayName, preferredUsername, u.Host)
}

func (h *Handler) getActorDisplayName(actor *ap.Actor) string {
	userName, _ := plain.FromHTML(actor.PreferredUsername)
	name, _ := plain.FromHTML(actor.Name)
	return h.getDisplayName(actor.ID, userName, name, actor.Type)
}

func (h *Handler) PrintNote(w text.Writer, r *Request, note *ap.Object, author *ap.Actor, sharer *ap.Actor, published time.Time, compact, printAuthor, printParentAuthor, titleIsLink bool) {
	if note.AttributedTo == "" {
		r.Log.Warn("Note has no author", "id", note.ID)
		return
	}

	maxLines := -1
	maxRunes := -1
	if compact {
		maxLines = h.Config.CompactViewMaxLines
		maxRunes = h.Config.CompactViewMaxRunes
	}

	noteBody := note.Content
	if compact {
		if note.Sensitive && note.Summary != "" {
			noteBody = fmt.Sprintf("[%s]", note.Summary)
		} else if note.Sensitive {
			noteBody = "[Content warning]"
		} else if note.Name != "" { // Page has a title, or this Note is a poll vote
			noteBody = note.Name
		} else if note.Summary != "" {
			noteBody = note.Summary
		}
	} else {
		if note.Sensitive && note.Summary != "" {
			noteBody = fmt.Sprintf("[%s]<br>%s", note.Summary, note.Content)
		} else if note.Sensitive {
			noteBody = "[Content warning]<br>" + note.Content
		} else if note.Name != "" && note.Content != "" {
			noteBody = fmt.Sprintf("%s<br>%s", note.Name, note.Content)
		} else if note.Name != "" {
			noteBody = note.Name
		}
	}

	contentLines, inlineLinks := getTextAndLinks(noteBody, maxRunes, maxLines)

	links := data.OrderedMap[string, string]{}

	if note.URL != "" {
		links.Store(note.URL, "")
	}

	hashtags := data.OrderedMap[string, string]{}
	mentionedUsers := ap.Audience{}

	for _, tag := range note.Tag {
		switch tag.Type {
		case ap.Hashtag:
			if tag.Name == "" {
				continue
			}
			if tag.Name[0] == '#' {
				hashtags.Store(strings.ToLower(tag.Name[1:]), tag.Name[1:])
			} else {
				hashtags.Store(strings.ToLower(tag.Name), tag.Name)
			}

		case ap.Mention:
			mentionedUsers.Add(tag.Href)

		case ap.Emoji:
			if tag.Icon != nil && tag.Name != "" && tag.Icon.URL != "" {
				links.Store(tag.Icon.URL, tag.Name)
			}

		default:
			r.Log.Warn("Skipping unsupported mention type", "post", note.ID, "type", tag.Type)
		}
	}

	for link, alt := range inlineLinks.All() {
		if !mentionedUsers.Contains(link) {
			links.Store(link, alt)
		}
	}

	for _, attachment := range note.Attachment {
		if attachment.URL != "" {
			links.Store(attachment.URL, "")
		} else if attachment.Href != "" {
			links.Store(attachment.Href, "")
		}
	}

	var replies int
	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where object->>'$.inReplyTo' = ?`, note.ID).Scan(&replies); err != nil {
		r.Log.Warn("Failed to count replies", "id", note.ID, "error", err)
	}

	authorDisplayName := author.PreferredUsername

	var title string
	if printAuthor && sharer == nil {
		title = fmt.Sprintf("%s %s", published.Format(time.DateOnly), authorDisplayName)
	} else if printAuthor && sharer != nil {
		title = fmt.Sprintf("%s %s ┃ 🔄 %s", published.Format(time.DateOnly), authorDisplayName, sharer.PreferredUsername)
	} else if sharer != nil {
		title = fmt.Sprintf("%s 🔄 %s", published.Format(time.DateOnly), sharer.PreferredUsername)
	} else {
		title = published.Format(time.DateOnly)
	}

	if note.Updated != nil && *note.Updated != (ap.Time{}) {
		title += " ┃ edited"
	}

	var parentAuthor sql.Null[ap.Actor]
	if note.InReplyTo != "" {
		if err := h.DB.QueryRowContext(r.Context, `select persons.actor from notes join persons on persons.id = notes.author where notes.id = ?`, note.InReplyTo).Scan(&parentAuthor); err != nil && errors.Is(err, sql.ErrNoRows) {
			r.Log.Info("Parent post or author is missing", "id", note.InReplyTo)
		} else if err != nil {
			r.Log.Warn("Failed to query parent post author", "id", note.InReplyTo, "error", err)
		}
	}

	if compact {
		meta := ""

		// show link # only if at least one link doesn't point to the post
		if note.URL == "" && len(links) > 0 {
			meta += fmt.Sprintf(" %d🔗", len(links))
		} else if note.URL != "" && len(links) > 1 {
			meta += fmt.Sprintf(" %d🔗", len(links)-1)
		}

		if len(hashtags) > 0 {
			meta += fmt.Sprintf(" %d#️", len(hashtags))
		}

		if len(mentionedUsers.OrderedMap) == 1 && (!parentAuthor.Valid || !mentionedUsers.Contains(parentAuthor.V.ID)) {
			meta += " 1👤"
		} else if len(mentionedUsers.OrderedMap) > 1 && (!parentAuthor.Valid || !mentionedUsers.Contains(parentAuthor.V.ID)) {
			meta += fmt.Sprintf(" %d👤", len(mentionedUsers.OrderedMap))
		} else if len(mentionedUsers.OrderedMap) > 1 && parentAuthor.Valid && mentionedUsers.Contains(parentAuthor.V.ID) {
			meta += fmt.Sprintf(" %d👤", len(mentionedUsers.OrderedMap)-1)
		}

		if replies > 0 {
			meta += fmt.Sprintf(" %d💬", replies)
		}

		if meta != "" {
			title += " ┃" + meta
		}
	}

	if printParentAuthor && parentAuthor.Valid && parentAuthor.V.PreferredUsername != "" {
		title += fmt.Sprintf(" ┃ RE: %s", parentAuthor.V.PreferredUsername)
	} else if printParentAuthor && note.InReplyTo != "" && (!parentAuthor.Valid || parentAuthor.V.PreferredUsername == "") {
		title += " ┃ RE: ?"
	}

	if !titleIsLink {
		w.Link(note.ID, title)
	} else if r.User == nil {
		w.Link("/view/"+strings.TrimPrefix(note.ID, "https://"), title)
	} else {
		w.Link("/users/view/"+strings.TrimPrefix(note.ID, "https://"), title)
	}

	for _, line := range contentLines {
		w.Quote(line)
	}

	if !compact {
		if r.User == nil {
			w.Link("/outbox/"+strings.TrimPrefix(author.ID, "https://"), authorDisplayName)
		} else {
			w.Link("/users/outbox/"+strings.TrimPrefix(author.ID, "https://"), authorDisplayName)
		}

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

		if r.User == nil && sharer != nil {
			links.Store("/outbox/"+strings.TrimPrefix(sharer.ID, "https://"), "🔄 "+sharer.PreferredUsername)
		} else if sharer != nil {
			links.Store("/users/outbox/"+strings.TrimPrefix(sharer.ID, "https://"), "🔄️ "+sharer.PreferredUsername)
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
						where shares.note = $1 and follows.follower = $2
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
				for rows.Next() {
					var sharerID, sharerName string
					if err := rows.Scan(&sharerID, &sharerName); err != nil {
						r.Log.Warn("Failed to scan sharer", "error", err)
						continue
					}
					links.Store("/users/outbox/"+strings.TrimPrefix(sharerID, "https://"), "🔄 "+sharerName)
				}
				rows.Close()
			}
		}

		for link, alt := range links.All() {
			if alt == "" {
				w.Link(link, link)
			} else {
				w.Link(link, alt)
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

		if r.User != nil && note.AttributedTo == r.User.ID && note.Type != ap.Question && note.Name == "" { // polls and votes cannot be edited
			w.Link("/users/edit/"+strings.TrimPrefix(note.ID, "https://"), "🩹 Edit")
			w.Link(fmt.Sprintf("titan://%s/users/upload/edit/%s", h.Domain, strings.TrimPrefix(note.ID, "https://")), "Upload edited post")
		}
		if r.User != nil && note.AttributedTo == r.User.ID {
			w.Link("/users/delete/"+strings.TrimPrefix(note.ID, "https://"), "💣 Delete")
		}
		if r.User != nil && note.Type == ap.Question && note.Closed == nil && (note.EndTime == nil || time.Now().Before(note.EndTime.Time)) {
			options := note.OneOf
			if len(options) == 0 {
				options = note.AnyOf
			}
			for _, option := range options {
				w.Linkf(fmt.Sprintf("/users/reply/%s?%s", strings.TrimPrefix(note.ID, "https://"), url.PathEscape(option.Name)), "📮 Vote %s", option.Name)
			}
		}

		if r.User != nil && note.IsPublic() && note.AttributedTo != r.User.ID {
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
			w.Link("/users/reply/"+strings.TrimPrefix(note.ID, "https://"), "💬 Reply")
			w.Link(fmt.Sprintf("titan://%s/users/upload/reply/%s", h.Domain, strings.TrimPrefix(note.ID, "https://")), "Upload reply")
		}
	}
}

func (h *Handler) PrintNotes(w text.Writer, r *Request, rows *sql.Rows, printParentAuthor, printDaySeparators bool, fallback string) int {
	var lastDay int64
	count := 0
	for rows.Next() {
		var note ap.Object
		var author sql.Null[ap.Actor]
		var sharer sql.Null[ap.Actor]
		var published int64
		if err := rows.Scan(&note, &author, &sharer, &published); err != nil {
			r.Log.Warn("Failed to scan post", "error", err)
			continue
		}

		if note.Type != ap.Note && note.Type != ap.Page && note.Type != ap.Article && note.Type != ap.Question {
			r.Log.Warn("Post type is unsupported", "type", note.Type)
			continue
		}

		if !author.Valid {
			r.Log.Warn("Post author is unknown", "note", note.ID, "author", note.AttributedTo)
			continue
		}

		currentDay := published / (60 * 60 * 24)

		if count > 0 && printDaySeparators && currentDay != lastDay {
			w.Separator()
		} else if count > 0 {
			w.Empty()
		}

		if sharer.Valid {
			h.PrintNote(w, r, &note, &author.V, &sharer.V, time.Unix(published, 0), true, true, printParentAuthor, true)
		} else {
			h.PrintNote(w, r, &note, &author.V, nil, time.Unix(published, 0), true, true, printParentAuthor, true)
		}

		lastDay = currentDay
		count++
	}

	if count == 0 {
		w.Text(fallback)
	}

	return count
}
