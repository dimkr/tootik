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
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/LukeEmmet/html2gemini"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/go-ap/activitypub"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	urlRegex = regexp.MustCompile(`\b(https|http|gemini|gopher|gophers):\/\/[-a-zA-Z0-9()!@:%_\+.~#?&\/\/=]+`)
	gmiOpts  *html2gemini.Options
)

func init() {
	handlers[regexp.MustCompile(`^/users/view$`)] = withUserMenu(view)
	handlers[regexp.MustCompile(`^/view$`)] = withUserMenu(view)

	gmiOpts = html2gemini.NewOptions()
	gmiOpts.OmitLinks = true
}

func getNativeLanguageValue(values activitypub.NaturalLanguageValues) string {
	if len(values) == 0 {
		return ""
	}

	// TODO: choose language?
	html := string(values[0].Value)

	ctx := html2gemini.NewTraverseContext(*gmiOpts)
	gmi, err := html2gemini.FromString(html, *ctx)
	if err != nil {
		log.WithField("html", html).WithError(err).Warn("Failed to convert HTML to gemtext")
		return html
	}

	return gmi
}

func getNoteContent(note *activitypub.Object) string {
	return getNativeLanguageValue(note.Content)
}

func getDisplayName(id, preferredUsername, name string) string {
	prefix := fmt.Sprintf("https://%s/user/", cfg.Domain)

	emoji := "🐘"
	isLocal := strings.HasPrefix(id, prefix)
	if isLocal {
		emoji = "🧑‍🚀"
	}

	displayName := preferredUsername
	if name != "" {
		displayName = name
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

func getActorDisplayName(actor *activitypub.Actor) string {
	return getDisplayName(string(actor.ID.GetLink()), getNativeLanguageValue(actor.PreferredUsername), getNativeLanguageValue(actor.Name))
}

func printNote(ctx context.Context, db *sql.DB, w io.Writer, actorID string, note *activitypub.Object, viewer *data.Object, compact bool) {
	if note.AttributedTo == nil {
		log.WithField("id", note.ID.GetLink()).Warn("Note has no author")
		return
	}

	authorID := string(note.AttributedTo.GetLink())

	authorDisplayName := authorID
	if author, err := fed.Resolve(ctx, db, viewer, authorID); err == nil {
		authorDisplayName = getActorDisplayName(author)
	}

	title := fmt.Sprintf("=> /view?%s %s %s", url.QueryEscape(string(note.ID.GetLink())), note.Published.Format(time.DateOnly), authorDisplayName)
	if viewer != nil {
		title = fmt.Sprintf("=> /users/view?%s %s %s", url.QueryEscape(string(note.ID.GetLink())), note.Published.Format(time.DateOnly), authorDisplayName)
	}

	parentID := ""
	if note.InReplyTo != nil && note.InReplyTo.IsLink() {
		parentID = string(note.InReplyTo.GetLink())

		parentObject, err := data.Objects.GetByID(parentID, db)
		if err == nil {
			parentNote := activitypub.Object{}
			if err := json.Unmarshal([]byte(parentObject.Object), &parentNote); err == nil {
				parentAuthorID := string(parentNote.AttributedTo.GetLink())

				parentDisplayName := parentAuthorID
				if parentAuthor, err := fed.Resolve(ctx, db, viewer, parentAuthorID); err == nil {
					parentDisplayName = getActorDisplayName(parentAuthor)
				} else {
					log.WithField("author", parentAuthorID).WithError(err).Warn("Failed to resolve parent author")
				}

				if viewer == nil {
					title = fmt.Sprintf("=> /view?%s %s %s (RE: %s)", url.QueryEscape(string(note.ID.GetLink())), parentNote.Published.Format(time.DateOnly), authorDisplayName, parentDisplayName)
				} else {
					title = fmt.Sprintf("=> /users/view?%s %s %s (RE: %s)", url.QueryEscape(string(note.ID.GetLink())), parentNote.Published.Format(time.DateOnly), authorDisplayName, parentDisplayName)
				}
			}
		}
	}

	w.Write([]byte(title))
	w.Write([]byte("\n"))

	content := getNoteContent(note)

	w.Write([]byte(content))
	w.Write([]byte{'\n'})

	links := map[string]struct{}{}

	if note.URL != nil && note.URL.IsLink() {
		links[string(note.URL.GetLink())] = struct{}{}
	}

	if note.Attachment != nil {
		attachments, ok := note.Attachment.(activitypub.ItemCollection)
		if ok {
			for _, attachment := range attachments {
				if attachment.IsLink() {
					links[string(attachment.GetLink())] = struct{}{}
				} else if attachmentObject, ok := attachment.(*activitypub.Object); ok {
					links[string(attachmentObject.URL.GetLink())] = struct{}{}
				} else {
					log.WithField("post", note.ID).Info("Skipping invalid attachment")
				}
			}
		} else {
			log.WithFields(log.Fields{"post": note.ID, "type": note.Attachment.GetType()}).Info("Bad attachment type")
		}
	}

	for _, link := range urlRegex.FindAllString(content, -1) {
		links[link] = struct{}{}
	}

	for link, _ := range links {
		fmt.Fprintf(w, "=> %s %s\n", link, link)
	}

	if !compact {
		hashtags := map[string]struct{}{}
		mentionedUsers := map[string]struct{}{}

		for _, tag := range note.Tag {
			o, ok := tag.(*activitypub.Link)
			if !ok {
				continue
			}

			if o.Type == "Hashtag" {
				hashtag := getNativeLanguageValue(o.Name)
				if hashtag == "" {
					continue
				}
				if hashtag[0] == '#' {
					hashtags[hashtag[1:]] = struct{}{}
				} else {
					hashtags[hashtag] = struct{}{}
				}
				continue
			}

			if o.Type == activitypub.MentionType {
				mentionedUsers[string(o.ID.GetLink())] = struct{}{}
			}
		}

		if viewer == nil {
			fmt.Fprintf(w, "=> /outbox/%x %s\n", sha256.Sum256([]byte(authorID)), authorDisplayName)
		} else {
			fmt.Fprintf(w, "=> /users/outbox/%x %s\n", sha256.Sum256([]byte(authorID)), authorDisplayName)
		}

		for hashtag, _ := range hashtags {
			fmt.Fprintf(w, "#️%s \n", hashtag)
		}

		for mentionString, _ := range mentionedUsers {
			tokens := strings.Split(mentionString, "@")
			if len(tokens) < 2 {
				tokens = []string{tokens[0], cfg.Domain}
			}

			mention, err := fed.Resolve(ctx, db, viewer, fmt.Sprintf("https://%s/users/%s", tokens[1], tokens[0]))
			if err != nil {
				log.WithField("mention", mention).WithError(err).Warn("Failed to resolve mentioned user")
				continue
			}

			mentionDisplayName := getActorDisplayName(mention)

			if viewer == nil {
				fmt.Fprintf(w, "=> /outbox/%x %s\n", sha256.Sum256([]byte(actorID)), mentionDisplayName)
			} else {
				fmt.Fprintf(w, "=> /users/outbox/%x %s\n", sha256.Sum256([]byte(actorID)), mentionDisplayName)
			}
		}

		if parentID != "" && viewer == nil {
			fmt.Fprintf(w, "=> /view?%s ⬆️ View parent\n", url.QueryEscape(parentID))
		} else if parentID != "" {
			fmt.Fprintf(w, "=> /users/view?%s ⬆️ View parent\n", url.QueryEscape(parentID))
		}
	}

	if viewer != nil {
		fmt.Fprintf(w, "=> /users/reply/%x ↩️ Reply\n", sha256.Sum256([]byte(note.ID.GetLink())))
	}
}

func view(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	if requestUrl.RawQuery == "" {
		w.Write([]byte("10 Post ID\r\n"))
		return
	}

	id, err := url.QueryUnescape(requestUrl.RawQuery)
	if err != nil {
		log.WithField("url", requestUrl.String()).WithError(err).Info("Failed to decode post ID")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	log.WithField("post", id).Info("Viewing post")

	o, err := data.Objects.GetByID(id, db)
	if err != nil {
		log.WithField("post", id).WithError(err).Info("Failed to find post")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	m := activitypub.Object{}
	if err := json.Unmarshal([]byte(o.Object), &m); err != nil {
		w.Write([]byte("40 Error\r\n"))
		return
	}

	w.Write([]byte("20 text/gemini\r\n"))
	printNote(ctx, db, w, o.ID, &m, user, false)
}
