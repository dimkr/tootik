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
	"github.com/dimkr/tootik/front/text/plain"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type noteMetadata struct {
	Author sql.NullString
	Sharer sql.NullString
}

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

func (h *Handler) getDisplayName(id, preferredUsername, name string, t ap.ActorType, log *slog.Logger) string {
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
		log.Warn("Failed to parse user ID", "id", id, "error", err)
		return fmt.Sprintf("%s %s", emoji, displayName)
	}

	return fmt.Sprintf("%s %s (%s@%s)", emoji, displayName, preferredUsername, u.Host)
}

func (h *Handler) getActorDisplayName(actor *ap.Actor, log *slog.Logger) string {
	userName, _ := plain.FromHTML(actor.PreferredUsername)
	name, _ := plain.FromHTML(actor.Name)
	return h.getDisplayName(actor.ID, userName, name, actor.Type, log)
}

func (r *request) PrintNote(w text.Writer, note *ap.Object, author *ap.Actor, sharer *ap.Actor, compact, printAuthor, printParentAuthor, titleIsLink bool) {
	if note.AttributedTo == "" {
		r.Log.Warn("Note has no author", "id", note.ID)
		return
	}

	maxLines := -1
	maxRunes := -1
	if compact {
		maxLines = r.Handler.Config.CompactViewMaxLines
		maxRunes = r.Handler.Config.CompactViewMaxRunes
	}

	noteBody := note.Content
	if note.Name != "" && note.Content != "" { // Page has a title
		noteBody = fmt.Sprintf("%s<br>%s", note.Name, note.Content)
	} else if note.Name != "" && note.Content == "" { // this Note is a poll vote
		noteBody = note.Name
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
		case ap.HashtagMention:
			if tag.Name == "" {
				continue
			}
			if tag.Name[0] == '#' {
				hashtags.Store(strings.ToLower(tag.Name[1:]), tag.Name[1:])
			} else {
				hashtags.Store(strings.ToLower(tag.Name), tag.Name)
			}

		case ap.MentionMention:
			mentionedUsers.Add(tag.Href)

		case ap.EmojiMention:
			if tag.Icon != nil && tag.Name != "" && tag.Icon.URL != "" {
				links.Store(tag.Icon.URL, tag.Name)
			}

		default:
			r.Log.Warn("Skipping unsupported mention type", "post", note.ID, "type", tag.Type)
		}
	}

	inlineLinks.Range(func(link, alt string) bool {
		if !mentionedUsers.Contains(link) {
			links.Store(link, alt)
		}
		return true
	})

	for _, attachment := range note.Attachment {
		if attachment.URL != "" {
			links.Store(attachment.URL, "")
		} else if attachment.Href != "" {
			links.Store(attachment.Href, "")
		}
	}

	var replies int
	if err := r.QueryRow(`select count(*) from notes where object->>'inReplyTo' = ?`, note.ID).Scan(&replies); err != nil {
		r.Log.Warn("Failed to count replies", "id", note.ID, "error", err)
	}

	authorDisplayName := author.PreferredUsername

	var title string
	if printAuthor && sharer == nil {
		title = fmt.Sprintf("%s %s", note.Published.Format(time.DateOnly), authorDisplayName)
	} else if printAuthor && sharer != nil {
		title = fmt.Sprintf("%s %s ┃ 🔁 %s", note.Published.Format(time.DateOnly), authorDisplayName, sharer.PreferredUsername)
	} else if sharer != nil {
		title = fmt.Sprintf("%s 🔁 %s", note.Published.Format(time.DateOnly), sharer.PreferredUsername)
	} else {
		title = note.Published.Format(time.DateOnly)
	}

	if note.Updated != nil && *note.Updated != (ap.Time{}) {
		title += " ┃ edited"
	}

	var parentAuthor ap.Actor
	if note.InReplyTo != "" {
		var parentAuthorString string
		if err := r.QueryRow(`select persons.actor from notes join persons on persons.id = notes.author where notes.id = ?`, note.InReplyTo).Scan(&parentAuthorString); err != nil && errors.Is(err, sql.ErrNoRows) {
			r.Log.Info("Parent post or author is missing", "id", note.InReplyTo)
		} else if err != nil {
			r.Log.Warn("Failed to query parent post author", "id", note.InReplyTo, "error", err)
		} else if err := json.Unmarshal([]byte(parentAuthorString), &parentAuthor); err != nil {
			r.Log.Warn("Failed to unmarshal parent post author", "id", note.InReplyTo, "error", err)
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

		if len(mentionedUsers.OrderedMap) == 1 && (parentAuthor.ID == "" || !mentionedUsers.Contains(parentAuthor.ID)) {
			meta += " 1👤"
		} else if len(mentionedUsers.OrderedMap) > 1 && (parentAuthor.ID == "" || !mentionedUsers.Contains(parentAuthor.ID)) {
			meta += fmt.Sprintf(" %d👤", len(mentionedUsers.OrderedMap))
		} else if len(mentionedUsers.OrderedMap) > 1 && parentAuthor.ID != "" && mentionedUsers.Contains(parentAuthor.ID) {
			meta += fmt.Sprintf(" %d👤", len(mentionedUsers.OrderedMap)-1)
		}

		if replies > 0 {
			meta += fmt.Sprintf(" %d💬", replies)
		}

		if meta != "" {
			title += " ┃" + meta
		}
	}

	if printParentAuthor && parentAuthor.PreferredUsername != "" {
		title += fmt.Sprintf(" ┃ RE: %s", parentAuthor.PreferredUsername)
	} else if printParentAuthor && note.InReplyTo != "" && parentAuthor.PreferredUsername == "" {
		title += " ┃ RE: ?"
	}

	if r.User != nil && ((len(note.To.OrderedMap) == 0 || len(note.To.OrderedMap) == 1 && note.To.Contains(r.User.ID)) && (len(note.CC.OrderedMap) == 0 || len(note.CC.OrderedMap) == 1 && note.CC.Contains(r.User.ID))) {
		title += " ┃ DM"
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

		for _, mentionID := range mentionedUsers.Keys() {
			var mentionUserName string
			if err := r.QueryRow(`select actor->>'preferredUsername' from persons where id = ?`, mentionID).Scan(&mentionUserName); err != nil && errors.Is(err, sql.ErrNoRows) {
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
			links.Store("/outbox/"+strings.TrimPrefix(sharer.ID, "https://"), "◀️ "+sharer.PreferredUsername)
		} else if sharer != nil {
			links.Store("/users/outbox/"+strings.TrimPrefix(sharer.ID, "https://"), "◀️ "+sharer.PreferredUsername)
		} else if note.IsPublic() && r.User != nil {
			var sharerID, sharerName string
			if err := r.QueryRow(
				`select persons.id, persons.actor->>'preferredUsername' from persons where id = ?`,
				note.Audience,
			).Scan(&sharerID, &sharerName); err != nil {
				r.Log.Warn("Failed to list sharer", "error", err)
			} else {
				links.Store("/users/outbox/"+strings.TrimPrefix(sharerID, "https://"), "◀️ "+sharerName)
			}
		}

		links.Range(func(link string, alt string) bool {
			if alt == "" {
				w.Link(link, link)
			} else {
				w.Link(link, alt)
			}
			return true
		})

		hashtags.Range(func(_ string, tag string) bool {
			var exists int
			if err := r.QueryRow(`select exists (select 1 from hashtags where hashtag = ? and note != ?)`, tag, note.ID).Scan(&exists); err != nil {
				r.Log.Warn("Failed to check if hashtag is used by other posts", "note", note.ID, "hashtag", tag)
				return true
			}

			if exists == 1 && r.User == nil {
				w.Linkf("/hashtag/"+tag, "Posts tagged #%s", tag)
			} else if exists == 1 {
				w.Linkf("/users/hashtag/"+tag, "Posts tagged #%s", tag)
			}

			return true
		})

		if r.User != nil && note.AttributedTo == r.User.ID && note.Type != ap.QuestionObject && note.Name == "" { // polls and votes cannot be edited
			w.Link("/users/edit/"+strings.TrimPrefix(note.ID, "https://"), "🩹 Edit")
		}
		if r.User != nil && note.AttributedTo == r.User.ID {
			w.Link("/users/delete/"+strings.TrimPrefix(note.ID, "https://"), "💣 Delete")
		}
		if r.User != nil && note.Type == ap.QuestionObject && note.Closed == nil && (note.EndTime == nil || time.Now().Before(note.EndTime.Time)) {
			options := note.OneOf
			if len(options) == 0 {
				options = note.AnyOf
			}
			for _, option := range options {
				w.Linkf(fmt.Sprintf("/users/reply/%s?%s", strings.TrimPrefix(note.ID, "https://"), url.PathEscape(option.Name)), "📮 Vote %s", option.Name)
			}
		}
		if r.User != nil {
			w.Link("/users/reply/"+strings.TrimPrefix(note.ID, "https://"), "💬 Reply")
		}

		if r.User != nil && note.IsPublic() && note.AttributedTo != r.User.ID {
			var shared int
			if err := r.QueryRow(`select exists (select 1 from shares where note = ? and by = ?)`, note.ID, r.User.ID).Scan(&shared); err != nil {
				r.Log.Warn("Failed to check if post is shared", "id", note.ID, "error", err)
			} else if shared == 0 {
				w.Link("/users/share/"+strings.TrimPrefix(note.ID, "https://"), "🔁 Boost")
			} else {
				w.Link("/users/unshare/"+strings.TrimPrefix(note.ID, "https://"), "🔄️ Unshare")
			}
		}
	}
}

func (r *request) PrintNotes(w text.Writer, rows data.OrderedMap[string, noteMetadata], printParentAuthor, printDaySeparators bool) {
	var lastDay int64
	first := true
	rows.Range(func(noteString string, meta noteMetadata) bool {
		note := ap.Object{}
		if err := json.Unmarshal([]byte(noteString), &note); err != nil {
			r.Log.Warn("Failed to unmarshal post", "error", err)
			return true
		}

		if note.Type != ap.NoteObject && note.Type != ap.PageObject && note.Type != ap.ArticleObject && note.Type != ap.QuestionObject {
			r.Log.Warn("Post type is unsupported", "type", note.Type)
			return true
		}

		if !meta.Author.Valid {
			r.Log.Warn("Post author is unknown", "note", note.ID, "author", note.AttributedTo)
			return true
		}

		author := ap.Actor{}
		if err := json.Unmarshal([]byte(meta.Author.String), &author); err != nil {
			r.Log.Warn("Failed to unmarshal post author", "error", err)
			return true
		}

		sharer := ap.Actor{}
		if meta.Sharer.Valid {
			if err := json.Unmarshal([]byte(meta.Sharer.String), &sharer); err != nil {
				r.Log.Warn("Failed to unmarshal post sharer", "error", err)
				return true
			}
		}

		currentDay := note.Published.Unix() / (60 * 60 * 24)

		if !first && printDaySeparators && currentDay != lastDay {
			w.Separator()
		} else if !first {
			w.Empty()
		}

		if meta.Sharer.Valid {
			r.PrintNote(w, &note, &author, &sharer, true, true, printParentAuthor, true)
		} else {
			r.PrintNote(w, &note, &author, nil, true, true, printParentAuthor, true)
		}

		lastDay = currentDay
		first = false
		return true
	})
}
