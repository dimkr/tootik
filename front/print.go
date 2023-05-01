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
	"github.com/dimkr/tootik/text"
	"github.com/dimkr/tootik/text/plain"
	log "github.com/sirupsen/logrus"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	compactViewMaxRunes = 200
	compactViewMaxLines = 4
)

var verifiedRegex = regexp.MustCompile(`(\s*:[a-zA-Z0-9_]+:\s*)+`)

func getTextAndLinks(s string, maxRunes, maxLines int) (string, []string, []string) {
	raw, links := plain.FromHTML(s)

	if raw == "" {
		raw = "[no content]"
	}

	if maxRunes > 6 {
		if cut := text.WordWrap(raw, maxRunes-6, 1)[0]; len(cut) < len(raw) {
			raw = cut + " [...]"
		}
	}

	lines := strings.Split(raw, "\n")

	if maxLines > 0 && len(lines) > maxLines {
		for i := maxLines - 1; i >= 0; i-- {
			if i == 0 || strings.TrimSpace(lines[i]) != "" {
				lines[i+1] = "[...]"
				return raw, lines[:i+2], links
			}
		}
	}

	return raw, lines, links
}

func getDisplayName(id, preferredUsername, name string, t ap.ActorType) string {
	prefix := fmt.Sprintf("https://%s/user/", cfg.Domain)

	isLocal := strings.HasPrefix(id, prefix)

	emoji := "👽"
	if t != ap.Person {
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
		log.WithField("id", id).WithError(err).Warn("Failed to parse user ID")
		return fmt.Sprintf("%s %s", emoji, displayName)
	}

	return fmt.Sprintf("%s %s (%s@%s)", emoji, displayName, preferredUsername, u.Host)
}

func getActorDisplayName(actor *ap.Actor) string {
	userName, _ := plain.FromHTML(actor.PreferredUsername)
	name, _ := plain.FromHTML(actor.Name)
	return getDisplayName(actor.ID, userName, name, actor.Type)
}

func (r *request) PrintNote(w text.Writer, note *ap.Object, author *ap.Actor, compact, printAuthor, printParentAuthor, titleIsLink bool) {
	if note.AttributedTo == "" {
		r.Log.WithField("id", note.ID).Warn("Note has no author")
		return
	}

	maxLines := -1
	maxRunes := -1
	if compact {
		maxLines = compactViewMaxLines
		maxRunes = compactViewMaxRunes
	}

	content, contentLines, inlineLinks := getTextAndLinks(note.Content, maxRunes, maxLines)

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
			r.Log.WithField("type", tag.Type).Warn("Skipping unsupported mention type")
		}
	}

	links := data.OrderedMap[string, struct{}]{}

	if note.URL != "" {
		links.Store(note.URL, struct{}{})
	}

	for _, link := range inlineLinks {
		links.Store(link, struct{}{})
	}

	for link, _ := range plain.GetInlineLinks(content) {
		links.Store(link, struct{}{})
	}

	for _, attachment := range note.Attachment {
		if attachment.URL != "" {
			links.Store(attachment.URL, struct{}{})
		}
	}

	var replies int
	if err := r.QueryRow(`select count(*) from notes where object->>'inReplyTo' = ?`, note.ID).Scan(&replies); err != nil {
		r.Log.WithField("id", note.ID).WithError(err).Warn("Failed to count replies")
	}

	authorDisplayName := author.PreferredUsername

	var title string
	if printAuthor {
		title = fmt.Sprintf("%s %s", note.Published.Format(time.DateOnly), authorDisplayName)
	} else {
		title = note.Published.Format(time.DateOnly)
	}

	var parentAuthor ap.Actor
	if note.InReplyTo != "" {
		var parentAuthorString string
		if err := r.QueryRow(`select persons.actor from notes join persons on persons.id = notes.author where notes.id = ?`, note.InReplyTo).Scan(&parentAuthorString); err != nil && errors.Is(err, sql.ErrNoRows) {
			r.Log.WithField("id", note.InReplyTo).Info("Parent post or author is missing")
		} else if err != nil {
			r.Log.WithField("id", note.InReplyTo).WithError(err).Warn("Failed to query parent post author")
		} else if err := json.Unmarshal([]byte(parentAuthorString), &parentAuthor); err != nil {
			r.Log.WithField("id", note.InReplyTo).WithError(err).Warn("Failed to unmarshal parent post author")
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
		w.Text(title)
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
			mention, err := r.Resolve(mentionID)
			if err != nil {
				r.Log.WithField("mention", mentionID).WithError(err).Warn("Failed to resolve mentioned user")
				continue
			}

			if r.User == nil {
				w.Link(fmt.Sprintf("/outbox/%x", sha256.Sum256([]byte(mentionID))), mention.PreferredUsername)
			} else {
				w.Link(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(mentionID))), mention.PreferredUsername)
			}
		}

		hashtags.Range(func(_ string, tag string) bool {
			var exists int
			if err := r.QueryRow(`select exists (select 1 from hashtags where hashtag = ? and note != ?)`, tag, note.ID).Scan(&exists); err != nil {
				r.Log.WithFields(log.Fields{"note": note.ID, "hashtag": tag}).Warn("Failed to check if hashtag is used by other posts")
				return true
			}

			if exists == 1 && r.User == nil {
				w.Linkf("/hashtag/"+tag, "Posts tagged #%s", tag)
			} else if exists == 1 {
				w.Linkf("/users/hashtag/"+tag, "Posts tagged #%s", tag)
			}

			return true
		})

		if r.User != nil {
			w.Link(fmt.Sprintf("/users/reply/%x", sha256.Sum256([]byte(note.ID))), "💬 Reply")
		}
	}
}

func (r *request) PrintNotes(w text.Writer, rows data.OrderedMap[string, sql.NullString], printAuthor, printParentAuthor bool) {
	rows.Range(func(noteString string, actorString sql.NullString) bool {
		note := ap.Object{}
		if err := json.Unmarshal([]byte(noteString), &note); err != nil {
			r.Log.WithError(err).Warn("Failed to unmarshal post")
			return true
		}

		if note.Type != ap.NoteObject {
			r.Log.WithField("type", note.Type).Warn("Post is note a note")
			return true
		}

		if actorString.Valid && actorString.String != "" {
			author := ap.Actor{}
			if err := json.Unmarshal([]byte(actorString.String), &author); err != nil {
				r.Log.WithError(err).Warn("Failed to unmarshal post author")
				return true
			}

			r.PrintNote(w, &note, &author, true, printAuthor, printParentAuthor, true)
		} else {
			if author, err := r.Resolve(note.AttributedTo); err != nil {
				r.Log.WithFields(log.Fields{"note": note.ID, "author": note.AttributedTo}).WithError(err).Warn("Failed to resolve post author")
				return true
			} else {
				r.PrintNote(w, &note, author, true, printAuthor, printParentAuthor, true)
			}
		}

		if len(rows) > 1 {
			w.Empty()
		}

		return true
	})
}
