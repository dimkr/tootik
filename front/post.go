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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/inbox"
)

const (
	pollOptionsDelimeter = "|"
	pollMinOptions       = 2
)

var (
	mentionRegex = regexp.MustCompile(`\B@(\w+)(?:@((?:\w+\.)+\w+(?::\d{1,5}){0,1})){0,1}\b`)
	hashtagRegex = regexp.MustCompile(`\B#\w{1,32}\b`)
	pollRegex    = regexp.MustCompile(`^\[(?:(?i)POLL)\s+(.+)\s*\]\s*(.+)`)
)

func (h *Handler) post(w text.Writer, r *Request, oldNote *ap.Object, inReplyTo *ap.Object, quoteID string, to ap.Audience, cc ap.Audience, audience string, readInput inputFunc) {
	now := ap.Time{Time: time.Now()}

	if oldNote == nil {
		var today, last sql.NullInt64
		if err := h.DB.QueryRowContext(r.Context, `select count(*), max(inserted) from outbox where activity->>'$.actor' = $1 and sender = $1 and activity->>'$.type' = 'Create' and inserted > $2`, r.User.ID, now.Add(-24*time.Hour).Unix()).Scan(&today, &last); err != nil {
			r.Log.Warn("Failed to check if new post needs to be throttled", "error", err)
			w.Error()
			return
		}

		if today.Valid && today.Int64 >= h.Config.MaxPostsPerDay {
			r.Log.Warn("User has exceeded the daily posts quota", "posts", today.Int64)
			w.Status(40, "Reached daily posts quota")
			return
		}

		if today.Valid && last.Valid {
			t := time.Unix(last.Int64, 0)
			can := t.Add(max(1, time.Duration(today.Int64/h.Config.PostThrottleFactor)) * h.Config.PostThrottleUnit)
			until := time.Until(can)
			if until > 0 {
				r.Log.Warn("User is posting too frequently", "last", t, "can", can)
				w.Statusf(40, "Please wait for %s", until.Truncate(time.Second).String())
				return
			}
		}
	}

	content, ok := readInput()
	if !ok {
		return
	}

	if utf8.RuneCountInString(content) > h.Config.MaxPostsLength {
		w.Status(40, "Post is too long")
		return
	}

	var postID string
	if oldNote == nil {
		var err error
		postID, err = h.Inbox.NewID(r.User.ID, "post")
		if err != nil {
			r.Log.Error("Failed to generate post ID", "error", err)
			w.Error()
			return
		}
	} else {
		postID = oldNote.ID
	}

	var tags []ap.Tag

	for _, hashtag := range hashtagRegex.FindAllString(content, -1) {
		tags = append(tags, ap.Tag{Type: ap.Hashtag, Name: hashtag, Href: fmt.Sprintf("gemini://%s/hashtag/%s", h.Domain, hashtag[1:])})
	}

	for _, mention := range mentionRegex.FindAllStringSubmatch(content, -1) {
		if len(mention) < 3 {
			continue
		}
		var actorID string
		var err error
		if mention[2] == "" && inReplyTo != nil {
			err = h.DB.QueryRowContext(r.Context, `select id from (select id, case when id = $1 then 3 when id in (select followed from follows where follower = $2 and accepted = 1) then 2 when ed25519privkey is not null then 1 else 0 end as score from persons where actor->>'$.preferredUsername' = $3) where score > 0 order by score desc limit 1`, inReplyTo.AttributedTo, r.User.ID, mention[1]).Scan(&actorID)
		} else if mention[2] == "" && inReplyTo == nil {
			err = h.DB.QueryRowContext(r.Context, `select id from (select id, case when ed25519privkey is not null then 2 when id in (select followed from follows where follower = $1 and accepted = 1) then 1 else 0 end as score from persons where actor->>'$.preferredUsername' = $2) where score > 0 order by score desc limit 1`, r.User.ID, mention[1]).Scan(&actorID)
		} else {
			err = h.DB.QueryRowContext(r.Context, `select id from persons where actor->>'$.preferredUsername' = $1 and host = $2`, mention[1], mention[2]).Scan(&actorID)
		}

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				r.Log.Warn("Failed to guess mentioned actor ID", "mention", mention[0])
			} else {
				r.Log.Warn("Failed to guess mentioned actor ID", "mention", mention[0], "error", err)
			}
			continue
		}

		r.Log.Info("Adding mention", "name", mention[0], "actor", actorID)
		tags = append(tags, ap.Tag{Type: ap.Mention, Name: mention[0], Href: actorID})
		cc.Add(actorID)
	}

	note := ap.Object{
		Type:         ap.Note,
		ID:           postID,
		AttributedTo: r.User.ID,
		Content:      content,
		Published:    now,
		To:           to,
		CC:           cc,
		Audience:     audience,
		Tag:          tags,
		Quote:        quoteID,
	}

	anyRecipient := false

	if inReplyTo != nil {
		note.InReplyTo = inReplyTo.ID

		if inReplyTo.Sensitive {
			note.Sensitive = true
			note.Summary = inReplyTo.Summary
		}

		if inReplyTo.Type == ap.Question {
			options := inReplyTo.OneOf
			if len(options) == 0 {
				options = inReplyTo.AnyOf
			}

			for _, option := range options {
				if option.Name == note.Content {
					if inReplyTo.Closed != (ap.Time{}) || (inReplyTo.EndTime != (ap.Time{}) && time.Now().After(inReplyTo.EndTime.Time)) {
						w.Status(40, "Cannot vote in a closed poll")
						return
					}

					note.Content = ""
					note.Name = option.Name
					note.To = ap.Audience{}
					note.To.Add(inReplyTo.AttributedTo)
					note.CC = ap.Audience{}

					// allow users to vote on their own polls
					anyRecipient = true
				}
			}
		}
	}

	if !anyRecipient {
		for actorID := range note.To.Keys() {
			if actorID != r.User.ID {
				anyRecipient = true
				break
			}
		}
	}
	if !anyRecipient {
		for actorID := range note.CC.Keys() {
			if actorID != r.User.ID {
				anyRecipient = true
				break
			}
		}
	}
	if !anyRecipient {
		w.Status(40, "Post audience is empty")
		return
	}

	if len(note.To.OrderedMap)+len(note.CC.OrderedMap) > h.Config.MaxRecipients {
		w.Status(40, "Too many recipients")
		return
	}

	if m := pollRegex.FindStringSubmatchIndex(note.Content); m != nil {
		optionNames := strings.SplitN(note.Content[m[4]:], pollOptionsDelimeter, h.Config.PollMaxOptions+1)
		if len(optionNames) < pollMinOptions || len(optionNames) > h.Config.PollMaxOptions {
			r.Log.Info("Received invalid poll", "content", note.Content)
			w.Statusf(40, "Polls must have %d to %d options", pollMinOptions, h.Config.PollMaxOptions)
			return
		}

		note.AnyOf = make([]ap.PollOption, len(optionNames))

		for i, optionName := range optionNames {
			plainName, _ := plain.FromHTML(optionName)
			note.AnyOf[i].Name = strings.TrimSpace(plainName)

			if note.AnyOf[i].Name == "" {
				w.Status(40, "Poll option cannot be empty")
				return
			}
		}

		note.Type = ap.Question
		note.Content = note.Content[m[2]:m[3]]
		note.EndTime = ap.Time{Time: time.Now().Add(h.Config.PollDuration)}
	}

	if inReplyTo == nil || inReplyTo.Type != ap.Question {
		note.Content = plain.ToHTML(note.Content, note.Tag)
	}

	if note.IsPublic() {
		note.InteractionPolicy.CanQuote.AutomaticApproval.Add(ap.Public)
	}

	var err error
	if oldNote != nil {
		note.Published = oldNote.Published

		if !note.Sensitive && oldNote.Sensitive {
			note.Sensitive = true
			note.Summary = oldNote.Summary
		}

		note.Updated = now

		err = h.Inbox.UpdateNote(r.Context, h.DB, r.User, r.Keys[1], &note)
	} else {
		err = h.Inbox.Create(r.Context, h.Config, h.DB, &note, r.User, r.Keys[1])
	}
	if err != nil {
		r.Log.Error("Failed to insert post", "error", err)
		if errors.Is(err, inbox.ErrDeliveryQueueFull) {
			w.Status(40, "Please try again later")
		} else {
			w.Error()
		}
		return
	}

	if r.URL.Scheme == "titan" {
		w.Redirectf("gemini://%s/users/view/%s", h.Domain, strings.TrimPrefix(postID, "https://"))
	} else {
		w.Redirectf("/users/view/%s", strings.TrimPrefix(postID, "https://"))
	}
}
