/*
Copyright 2023 - 2025 Dima Krasner

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
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
)

var verifiedRegex = regexp.MustCompile(`(\s*:[a-zA-Z0-9_]+:\s*)+`)

type metaBuilder struct {
	w   io.Writer
	sep bool
}

func (b *metaBuilder) Write(p []byte) (int, error) {
	if !b.sep {
		b.w.Write(danger.Bytes(" ┃"))
		b.sep = true
	}

	return b.w.Write(p)
}

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

	if maxLines <= 0 {
		return strings.Split(raw, "\n"), links
	}

	lines := strings.SplitN(raw, "\n", maxLines+1)

	if len(lines) <= maxLines {
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
	origin, err := ap.Origin(id)
	if err != nil {
		slog.Warn("Failed to get origin of actor", "id", id, "error", err)
		origin = ""
	}

	emoji := "👽"
	if t == ap.Group {
		emoji = "👥"
	} else if t != ap.Person {
		emoji = "🤖"
	} else if strings.HasPrefix(origin, "did:") {
		emoji = "🚴"
	} else if origin == h.Domain {
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

func (h *Handler) getCompactNoteContent(note *ap.Object) ([]string, data.OrderedMap[string, string]) {
	noteBody := note.Content
	if note.Sensitive && note.Summary != "" {
		noteBody = fmt.Sprintf("[%s]", note.Summary)
	} else if note.Sensitive {
		noteBody = "[Content warning]"
	} else if note.Name != "" { // Page has a title, or this Note is a poll vote
		noteBody = note.Name
	} else if note.Summary != "" {
		noteBody = note.Summary
	}

	return getTextAndLinks(noteBody, h.Config.CompactViewMaxRunes, h.Config.CompactViewMaxLines)
}

func (h *Handler) getNoteContent(note *ap.Object, compact bool) ([]string, data.OrderedMap[string, string], data.OrderedMap[string, string], ap.Audience) {
	var content []string
	var inlineLinks data.OrderedMap[string, string]
	if compact {
		content, inlineLinks = h.getCompactNoteContent(note)
	} else {
		noteBody := note.Content

		if note.Sensitive && note.Summary != "" {
			noteBody = fmt.Sprintf("[%s]<br>%s", note.Summary, note.Content)
		} else if note.Sensitive {
			noteBody = "[Content warning]<br>" + note.Content
		} else if note.Name != "" && note.Content != "" {
			noteBody = fmt.Sprintf("%s<br>%s", note.Name, note.Content)
		} else if note.Name != "" {
			noteBody = note.Name
		}

		content, inlineLinks = getTextAndLinks(noteBody, -1, -1)
	}

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
			slog.Warn("Skipping unsupported mention type", "post", note.ID, "type", tag.Type)
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

	return content, links, hashtags, mentionedUsers
}

func (h *Handler) printCompactNote(w text.Writer, r *Request, note *ap.Object, author *ap.Actor, sharer *ap.Actor, published time.Time, printParentAuthor bool) {
	if note.AttributedTo == "" {
		r.Log.Warn("Note has no author", "id", note.ID)
		return
	}

	contentLines, links, hashtags, mentionedUsers := h.getNoteContent(note, true)

	var replies int
	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where object->>'$.inReplyTo' = ?`, note.ID).Scan(&replies); err != nil {
		r.Log.Warn("Failed to count replies", "id", note.ID, "error", err)
	}

	var quotes int
	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where object->>'$.quote' = ?`, note.ID).Scan(&quotes); err != nil {
		r.Log.Warn("Failed to count quotes", "id", note.ID, "error", err)
	}

	authorDisplayName := author.PreferredUsername

	var title strings.Builder
	if sharer != nil {
		fmt.Fprintf(&title, "%s %s ┃ 🔄 %s", published.Format(time.DateOnly), authorDisplayName, sharer.PreferredUsername)
	} else {
		fmt.Fprintf(&title, "%s %s", published.Format(time.DateOnly), authorDisplayName)
	}

	if note.Updated != (ap.Time{}) {
		title.WriteString(" ┃ edited")
	}

	var parentAuthor sql.Null[ap.Actor]
	if note.InReplyTo != "" {
		if err := h.DB.QueryRowContext(r.Context, `select json(persons.actor) from notes join persons on persons.id = notes.author where notes.id = ?`, note.InReplyTo).Scan(&parentAuthor); err != nil && errors.Is(err, sql.ErrNoRows) {
			r.Log.Info("Parent post or author is missing", "id", note.InReplyTo)
		} else if err != nil {
			r.Log.Warn("Failed to query parent post author", "id", note.InReplyTo, "error", err)
		}
	}

	meta := metaBuilder{w: &title}

	// show link # only if at least one link doesn't point to the post
	if note.URL == "" && len(links) > 0 {
		fmt.Fprintf(&meta, " %d🔗", len(links))
	} else if note.URL != "" && len(links) > 1 {
		fmt.Fprintf(&meta, " %d🔗", len(links)-1)
	}

	if len(hashtags) > 0 {
		fmt.Fprintf(&meta, " %d#️", len(hashtags))
	}

	if len(mentionedUsers.OrderedMap) == 1 && (!parentAuthor.Valid || !mentionedUsers.Contains(parentAuthor.V.ID)) {
		meta.Write(danger.Bytes(" 1👤"))
	} else if len(mentionedUsers.OrderedMap) > 1 && (!parentAuthor.Valid || !mentionedUsers.Contains(parentAuthor.V.ID)) {
		fmt.Fprintf(&meta, " %d👤", len(mentionedUsers.OrderedMap))
	} else if len(mentionedUsers.OrderedMap) > 1 && parentAuthor.Valid && mentionedUsers.Contains(parentAuthor.V.ID) {
		fmt.Fprintf(&meta, " %d👤", len(mentionedUsers.OrderedMap)-1)
	}

	if replies > 0 {
		fmt.Fprintf(&meta, " %d💬", replies)
	}

	if quotes > 0 {
		fmt.Fprintf(&meta, " %d♻️", quotes)
	}

	if printParentAuthor && parentAuthor.Valid && parentAuthor.V.PreferredUsername != "" {
		fmt.Fprintf(&title, " ┃ RE: %s", parentAuthor.V.PreferredUsername)
	} else if printParentAuthor && note.InReplyTo != "" && (!parentAuthor.Valid || parentAuthor.V.PreferredUsername == "") {
		title.WriteString(" ┃ RE: ?")
	}

	if r.User == nil {
		w.Link("/view/"+strings.TrimPrefix(note.ID, "https://"), title.String())
	} else {
		w.Link("/users/view/"+strings.TrimPrefix(note.ID, "https://"), title.String())
	}

	for _, line := range contentLines {
		w.Quote(line)
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
			h.printCompactNote(w, r, &note, &author.V, &sharer.V, time.Unix(published, 0), printParentAuthor)
		} else {
			h.printCompactNote(w, r, &note, &author.V, nil, time.Unix(published, 0), printParentAuthor)
		}

		lastDay = currentDay
		count++
	}

	if count == 0 {
		w.Text(fallback)
	}

	return count
}
