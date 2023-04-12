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

package gem

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/gmi"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const compactViewMaxLines = 4

var (
	urlRegex      = regexp.MustCompile(`\b(https|http|gemini|gopher|gophers):\/\/[-a-zA-Z0-9()!@:%_\+.~#?&\/\/=]+`)
	verifiedRegex = regexp.MustCompile(`(\s*:[a-zA-Z0-9_]+:\s*)+`)
)

func getQuoteAndLinks(s string, maxLines int) (string, []string) {
	text, links := gmi.FromHTML(s)

	if maxLines > 0 {
		lines := strings.Split(text, "\n")
		if len(lines) > maxLines {
			text = strings.TrimRight(strings.Join(lines[:maxLines], "\n"), "\n") + "\n[...]"
		}
	}

	return gmi.Quote(text), links
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
	userName, _ := gmi.FromHTML(actor.PreferredUsername)
	name, _ := gmi.FromHTML(actor.Name)
	return getDisplayName(actor.ID, userName, name, actor.Type)
}

func printNote(w io.Writer, r *request, note *ap.Object, author *ap.Actor, compact, printAuthor, printParentAuthor bool) {
	if note.AttributedTo == "" {
		r.Log.WithField("id", note.ID).Warn("Note has no author")
		return
	}

	links := data.OrderedMap[string, struct{}]{}

	if note.URL != "" {
		links.Store(note.URL, struct{}{})
	}

	for _, attachment := range note.Attachment {
		if attachment.URL != "" {
			links.Store(attachment.URL, struct{}{})
		}
	}

	maxLines := -1
	if compact {
		maxLines = compactViewMaxLines
	}

	content, inlineLinks := getQuoteAndLinks(note.Content, maxLines)

	for _, link := range inlineLinks {
		links.Store(link, struct{}{})
	}

	for _, link := range urlRegex.FindAllString(content, -1) {
		links.Store(link, struct{}{})
	}

	hashtags := map[string]struct{}{}
	mentionedUsers := map[string]struct{}{}

	for _, tag := range note.Tag {
		switch tag.Type {
		case ap.HashtagMention:
			if tag.Name == "" {
				continue
			}
			if tag.Name[0] == '#' {
				hashtags[tag.Name[1:]] = struct{}{}
			} else {
				hashtags[tag.Name] = struct{}{}
			}

		case ap.MentionMention:
			mentionedUsers[tag.Href] = struct{}{}

		default:
			r.Log.WithField("type", tag.Type).Warn("Skipping unsupported mention type")
		}
	}

	var replies int
	if err := r.QueryRow(`select count(*) from notes where object->>'inReplyTo' = ?`, note.ID).Scan(&replies); err != nil {
		r.Log.WithField("id", note.ID).WithError(err).Warn("Failed to count replies")
	}

	authorDisplayName := getActorDisplayName(author)

	var title string
	if r.User != nil && printAuthor {
		title = fmt.Sprintf("=> /users/view/%x %s %s", sha256.Sum256([]byte(note.ID)), note.Published.Format(time.DateOnly), authorDisplayName)
	} else if r.User != nil && !printAuthor {
		title = fmt.Sprintf("=> /users/view/%x %s", sha256.Sum256([]byte(note.ID)), note.Published.Format(time.DateOnly))
	} else if r.User == nil && printAuthor {
		title = fmt.Sprintf("=> /view/%x %s %s", sha256.Sum256([]byte(note.ID)), note.Published.Format(time.DateOnly), authorDisplayName)
	} else if r.User == nil && !printAuthor {
		title = fmt.Sprintf("=> /view/%x %s", sha256.Sum256([]byte(note.ID)), note.Published.Format(time.DateOnly))
	}

	if len(links) > 0 || len(hashtags) > 0 || len(mentionedUsers) > 0 || replies > 0 {
		title += " ┃"
	}

	if len(links) > 0 {
		title += fmt.Sprintf(" %d🔗", len(links))
	}

	if len(hashtags) > 0 {
		title += fmt.Sprintf(" %d#️", len(hashtags))
	}

	if len(mentionedUsers) > 0 {
		title += fmt.Sprintf(" %d👤", len(mentionedUsers))
	}

	if replies > 0 {
		title += fmt.Sprintf(" %d💬", replies)
	}

	if printParentAuthor && note.InReplyTo != "" {
		var parentAuthorString string
		var parentAuthor ap.Actor
		if err := r.QueryRow(`select persons.actor from notes join persons on persons.id = notes.author where notes.id = ?`, note.InReplyTo).Scan(&parentAuthorString); err != nil && errors.Is(err, sql.ErrNoRows) {
			r.Log.WithField("id", note.InReplyTo).Info("Parent post or author is missing")
		} else if err != nil {
			r.Log.WithField("id", note.InReplyTo).WithError(err).Warn("Failed to query parent post author")
		} else if err := json.Unmarshal([]byte(parentAuthorString), &parentAuthor); err != nil {
			r.Log.WithField("id", note.InReplyTo).WithError(err).Warn("Failed to unmarshal parent post author")
		} else if compact {
			title += fmt.Sprintf(" ┃ RE: %s", parentAuthor.PreferredUsername)
		} else {
			title += fmt.Sprintf(" ┃ RE: %s", getActorDisplayName(&parentAuthor))
		}
	}

	w.Write([]byte(title))
	w.Write([]byte("\n"))

	w.Write([]byte(content))
	if content != "" && content[len(content)-1] != '\n' {
		w.Write([]byte{'\n'})
	}

	if !compact {
		links.Range(func(link string, _ struct{}) bool {
			fmt.Fprintf(w, "=> %s %s\n", link, link)
			return true
		})

		if r.User == nil {
			fmt.Fprintf(w, "=> /outbox/%x %s\n", sha256.Sum256([]byte(author.ID)), authorDisplayName)
		} else {
			fmt.Fprintf(w, "=> /users/outbox/%x %s\n", sha256.Sum256([]byte(author.ID)), authorDisplayName)
		}

		for hashtag, _ := range hashtags {
			fmt.Fprintf(w, "#️%s \n", hashtag)
		}

		for mentionID, _ := range mentionedUsers {
			mention, err := r.Resolve(mentionID)
			if err != nil {
				r.Log.WithField("mention", mentionID).WithError(err).Warn("Failed to resolve mentioned user")
				continue
			}

			mentionDisplayName := getActorDisplayName(mention)

			if r.User == nil {
				fmt.Fprintf(w, "=> /outbox/%x %s\n", sha256.Sum256([]byte(mentionID)), mentionDisplayName)
			} else {
				fmt.Fprintf(w, "=> /users/outbox/%x %s\n", sha256.Sum256([]byte(mentionID)), mentionDisplayName)
			}
		}
	}

	if r.User != nil {
		fmt.Fprintf(w, "=> /users/reply/%x 💬 Reply\n", sha256.Sum256([]byte(note.ID)))
	}
}
