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
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/outbox"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
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

func (h *Handler) post(w text.Writer, r *request, oldNote *ap.Object, inReplyTo *ap.Object, to ap.Audience, cc ap.Audience, audience, prompt string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := ap.Time{Time: time.Now()}

	if oldNote == nil {
		var today, last sql.NullInt64
		if err := r.QueryRow(`select count(*), max(inserted) from outbox where activity->>'$.actor' = $1 and sender = $1 and activity->>'$.type' = 'Create' and inserted > $2`, r.User.ID, now.Add(-24*time.Hour).Unix()).Scan(&today, &last); err != nil {
			r.Log.Warn("Failed to check if new post needs to be throttled", "error", err)
			w.Error()
			return
		}

		if today.Valid && today.Int64 >= h.Config.MaxPostsPerDay {
			r.Log.Warn("User has exceeded the daily posts quota", "posts", today.Int64)
			w.Status(40, "Please wait before posting again")
			return
		}

		if today.Valid && last.Valid {
			t := time.Unix(last.Int64, 0)
			interval := max(1, time.Duration(today.Int64/h.Config.PostThrottleFactor)) * h.Config.PostThrottleUnit
			if now.Sub(t) < interval {
				r.Log.Warn("User is posting too frequently", "last", t, "can", t.Add(interval))
				w.Status(40, "Please wait before posting again")
				return
			}
		}
	}

	if r.URL.RawQuery == "" {
		w.Status(10, prompt)
		return
	}

	content, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		w.Error()
		return
	}

	if utf8.RuneCountInString(content) > h.Config.MaxPostsLength {
		w.Status(40, "Post is too long")
		return
	}

	var postID string
	if oldNote == nil {
		postID = fmt.Sprintf("https://%s/post/%x", h.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", r.User.ID, content, now.Unix()))))
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
			err = r.QueryRow(`select id from (select id, case when id = $1 then 3 when id in (select followed from follows where follower = $2 and accepted = 1) then 2 when host = $3 then 1 else 0 end as score from persons where actor->>'$.preferredUsername' = $4) where score > 0 order by score desc limit 1`, inReplyTo.AttributedTo, r.User.ID, h.Domain, mention[1]).Scan(&actorID)
		} else if mention[2] == "" && inReplyTo == nil {
			err = r.QueryRow(`select id from (select id, case when host = $1 then 2 when id in (select followed from follows where follower = $2 and accepted = 1) then 1 else 0 end as score from persons where actor->>'$.preferredUsername' = $3) where score > 0 order by score desc limit 1`, h.Domain, r.User.ID, mention[1]).Scan(&actorID)
		} else {
			err = r.QueryRow(`select id from persons where actor->>'$.preferredUsername' = $1 and host = $2`, mention[1], mention[2]).Scan(&actorID)
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
	}

	if inReplyTo != nil {
		note.InReplyTo = inReplyTo.ID

		if inReplyTo.Type == ap.Question {
			options := inReplyTo.OneOf
			if len(options) == 0 {
				options = inReplyTo.AnyOf
			}

			for _, option := range options {
				if option.Name == note.Content {
					if inReplyTo.Closed != nil || inReplyTo.EndTime != nil && time.Now().After(inReplyTo.EndTime.Time) {
						w.Status(40, "Cannot vote in a closed poll")
						return
					}

					note.Content = ""
					note.Name = option.Name
					note.To = ap.Audience{}
					note.To.Add(inReplyTo.AttributedTo)
					note.CC = ap.Audience{}
				}
			}
		}
	}

	anyRecipient := false
	note.To.Range(func(actorID string, _ struct{}) bool {
		if actorID != r.User.ID {
			anyRecipient = true
			return false
		}
		return true
	})
	if !anyRecipient {
		note.CC.Range(func(actorID string, _ struct{}) bool {
			if actorID != r.User.ID {
				anyRecipient = true
				return false
			}
			return true
		})
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
		endTime := ap.Time{Time: time.Now().Add(h.Config.PollDuration)}
		note.EndTime = &endTime
	}

	if inReplyTo == nil || inReplyTo.Type != ap.Question {
		note.Content = plain.ToHTML(note.Content, note.Tag)
	}

	if oldNote != nil {
		note.Published = oldNote.Published
		note.Updated = &now
		err = outbox.UpdateNote(r.Context, h.Domain, h.Config, r.Log, r.DB, &note)
	} else {
		err = outbox.Create(r.Context, h.Domain, h.Config, r.Log, r.DB, &note, r.User)
	}
	if err != nil {
		r.Log.Error("Failed to insert post", "error", err)
		if errors.Is(err, outbox.ErrDeliveryQueueFull) {
			w.Status(40, "Please try again later")
		} else {
			w.Error()
		}
		return
	}

	w.Redirectf("/users/view/%s", strings.TrimPrefix(postID, "https://"))
}
