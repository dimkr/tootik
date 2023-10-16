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
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	compactViewMaxRunes = 200
	compactViewMaxLines = 4
)

type noteMetadata struct {
	Author sql.NullString
	Group  sql.NullString
}

var verifiedRegex = regexp.MustCompile(`(\s*:[a-zA-Z0-9_]+:\s*)+`)

func getTextAndLinks(s string, maxRunes, maxLines int) ([]string, []string) {
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

	if maxLines > 0 && len(lines) > maxLines {
		for i := maxLines - 1; i >= 0; i-- {
			if i == 0 || strings.TrimSpace(lines[i]) != "" {
				lines[i+1] = "[…]"
				return lines[:i+2], links
			}
		}
	}

	return lines, links
}

func getDisplayName(id, preferredUsername, name string, t ap.ActorType, log *slog.Logger) string {
	prefix := fmt.Sprintf("https://%s/user/", cfg.Domain)

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

	if isLocal {
		return fmt.Sprintf("%s %s", emoji, displayName)
	}

	u, err := url.Parse(id)
	if err != nil {
		log.Warn("Failed to parse user ID", "id", id, "error", err)
		return fmt.Sprintf("%s %s", emoji, displayName)
	}

	return fmt.Sprintf("%s %s (%s@%s)", emoji, displayName, preferredUsername, u.Host)
}

func getActorDisplayName(actor *ap.Actor, log *slog.Logger) string {
	userName, _ := plain.FromHTML(actor.PreferredUsername)
	name, _ := plain.FromHTML(actor.Name)
	return getDisplayName(actor.ID, userName, name, actor.Type, log)
}

func (r *request) PrintNote(w text.Writer, note *ap.Object, author *ap.Actor, group *ap.Actor, compact, printAuthor, printParentAuthor, titleIsLink bool) {
	if note.AttributedTo == "" {
		r.Log.Warn("Note has no author", "id", note.ID)
		return
	}

	maxLines := -1
	maxRunes := -1
	if compact {
		maxLines = compactViewMaxLines
		maxRunes = compactViewMaxRunes
	}

	noteBody := note.Content
	if note.Name != "" && note.Content != "" { // Page has a title
		noteBody = fmt.Sprintf("%s<br>%s", note.Name, note.Content)
	} else if note.Name != "" && note.Content == "" { // this Note is a poll vote
		noteBody = note.Name
	}

	contentLines, inlineLinks := getTextAndLinks(noteBody, maxRunes, maxLines)

	hashtags := data.OrderedMap[string, string]{}
	mentionedUsers := data.OrderedMap[string, struct{}]{}

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
			mentionedUsers.Store(tag.Href, struct{}{})

		default:
			r.Log.Warn("Skipping unsupported mention type", "post", note.ID, "type", tag.Type)
		}
	}

	links := data.OrderedMap[string, struct{}]{}

	if note.URL != "" {
		links.Store(note.URL, struct{}{})
	}

	for _, link := range inlineLinks {
		links.Store(link, struct{}{})
	}

	for _, attachment := range note.Attachment {
		if attachment.URL != "" {
			links.Store(attachment.URL, struct{}{})
		} else if attachment.Href != "" {
			links.Store(attachment.Href, struct{}{})
		}
	}

	var replies int
	if err := r.QueryRow(`select count(*) from notes where object->>'inReplyTo' = ?`, note.ID).Scan(&replies); err != nil {
		r.Log.Warn("Failed to count replies", "id", note.ID, "error", err)
	}

	authorDisplayName := author.PreferredUsername

	var title string
	if printAuthor && group == nil {
		title = fmt.Sprintf("%s %s", note.Published.Format(time.DateOnly), authorDisplayName)
	} else if printAuthor && group != nil {
		title = fmt.Sprintf("%s %s ┃ 👥 %s", note.Published.Format(time.DateOnly), authorDisplayName, group.PreferredUsername)
	} else if group != nil {
		title = fmt.Sprintf("%s 👥 %s", note.Published.Format(time.DateOnly), group.PreferredUsername)
	} else {
		title = note.Published.Format(time.DateOnly)
	}

	if note.Updated != nil && *note.Updated != (time.Time{}) {
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

		if len(mentionedUsers) == 1 && (parentAuthor.ID == "" || !mentionedUsers.Contains(parentAuthor.ID)) {
			meta += " 1👤"
		} else if len(mentionedUsers) > 1 && (parentAuthor.ID == "" || !mentionedUsers.Contains(parentAuthor.ID)) {
			meta += fmt.Sprintf(" %d👤", len(mentionedUsers))
		} else if len(mentionedUsers) > 1 && parentAuthor.ID != "" && mentionedUsers.Contains(parentAuthor.ID) {
			meta += fmt.Sprintf(" %d👤", len(mentionedUsers)-1)
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
		w.Link(fmt.Sprintf("/view/%x", sha256.Sum256([]byte(note.ID))), title)
	} else {
		w.Link(fmt.Sprintf("/users/view/%x", sha256.Sum256([]byte(note.ID))), title)
	}

	for _, line := range contentLines {
		w.Quote(line)
	}

	if !compact {
		links.Range(func(link string, _ struct{}) bool {
			w.Link(link, link)
			return true
		})

		if r.User == nil {
			w.Link(fmt.Sprintf("/outbox/%x", sha256.Sum256([]byte(author.ID))), authorDisplayName)
		} else {
			w.Link(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(author.ID))), authorDisplayName)
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
				w.Link(fmt.Sprintf("/outbox/%x", sha256.Sum256([]byte(mentionID))), mentionUserName)
			} else {
				w.Link(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(mentionID))), mentionUserName)
			}
		}

		if r.User == nil && group != nil {
			w.Linkf(fmt.Sprintf("/outbox/%x", sha256.Sum256([]byte(group.ID))), "👥 %s", group.PreferredUsername)
		} else if group != nil {
			w.Linkf(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(group.ID))), "👥 %s", group.PreferredUsername)
		}

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

		if r.User != nil && note.AttributedTo == r.User.ID && note.Name == "" { // poll votes cannot be edited
			w.Link(fmt.Sprintf("/users/edit/%x", sha256.Sum256([]byte(note.ID))), "🩹 Edit")
		}
		if r.User != nil && note.AttributedTo == r.User.ID {
			w.Link(fmt.Sprintf("/users/delete/%x", sha256.Sum256([]byte(note.ID))), "💣 Delete")
		}
		if r.User != nil && note.Type == ap.QuestionObject && note.Closed == nil && (note.EndTime == nil || time.Now().Before(*note.EndTime)) {
			options := note.OneOf
			if len(options) == 0 {
				options = note.AnyOf
			}
			for _, option := range options {
				w.Linkf(fmt.Sprintf("/users/reply/%x?%s", sha256.Sum256([]byte(note.ID)), url.PathEscape(option.Name)), "📮 Vote %s", option.Name)
			}
		}
		if r.User != nil {
			w.Link(fmt.Sprintf("/users/reply/%x", sha256.Sum256([]byte(note.ID))), "💬 Reply")
		}
	}
}

func (r *request) PrintNotes(w text.Writer, rows data.OrderedMap[string, noteMetadata], printAuthor, printParentAuthor bool) {
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

		group := ap.Actor{}
		if meta.Group.Valid {
			if err := json.Unmarshal([]byte(meta.Group.String), &group); err != nil {
				r.Log.Warn("Failed to unmarshal post group", "error", err)
				return true
			}
		}

		if !first {
			w.Empty()
		}

		if meta.Group.Valid {
			r.PrintNote(w, &note, &author, &group, true, printAuthor, printParentAuthor, true)
		} else {
			r.PrintNote(w, &note, &author, nil, true, printAuthor, printParentAuthor, true)
		}

		first = false
		return true
	})
}
