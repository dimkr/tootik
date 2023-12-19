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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/outbox"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	pollOptionsDelimeter = "|"
	pollMinOptions       = 2
	pollMaxOptions       = 5
	pollDuration         = time.Hour * 24 * 30
)

var (
	mentionRegex = regexp.MustCompile(`\B@(\w+)(?:@(\w+\.\w+(?::\d+){0,1})){0,1}\b`)
	hashtagRegex = regexp.MustCompile(`\B#\w{1,32}\b`)
	pollRegex    = regexp.MustCompile(`^\[(?:(?i)POLL)\s+(.+)\s*\]\s*(.+)`)
)

func post(w text.Writer, r *request, inReplyTo *ap.Object, to ap.Audience, cc ap.Audience, prompt string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := time.Now()

	var today, last sql.NullInt64
	if err := r.QueryRow(`select count(*), max(inserted) from outbox where activity->>'actor' = ? and activity->>'type' = 'Create' and inserted > ?`, r.User.ID, now.Add(-24*time.Hour).Unix()).Scan(&today, &last); err != nil {
		r.Log.Warn("Failed to check if new post needs to be throttled", "error", err)
		w.Error()
		return
	}

	if today.Valid && today.Int64 >= 30 {
		r.Log.Warn("User has exceeded the daily posts quota", "posts", today.Int64)
		w.Status(40, "Please wait before posting again")
		return
	}

	if today.Valid && last.Valid {
		t := time.Unix(last.Int64, 0)
		interval := max(1, time.Duration(today.Int64/2)) * time.Minute
		if now.Sub(t) < interval {
			r.Log.Warn("User is posting too frequently", "last", t, "can", t.Add(interval))
			w.Status(40, "Please wait before posting again")
			return
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

	if len(content) > cfg.MaxPostsLength {
		w.Status(40, "Post is too long")
		return
	}

	postID := fmt.Sprintf("https://%s/post/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", r.User.ID, content, now.Unix()))))

	tags := ap.Mentions{}

	for _, hashtag := range hashtagRegex.FindAllString(content, -1) {
		tags = append(tags, ap.Mention{Type: ap.HashtagMention, Name: hashtag, Href: fmt.Sprintf("gemini://%s/hashtag/%s", cfg.Domain, hashtag[1:])})
	}

	for _, mention := range mentionRegex.FindAllStringSubmatch(content, -1) {
		fmt.Println(content)
		fmt.Println(mention)
		if len(mention) < 3 {
			continue
		}
		var actorID string
		var err error
		if mention[2] == "" && inReplyTo != nil {
			err = r.QueryRow(`select id from (select id, case when id = $1 then 3 when id in (select followed from follows where follower = $2 and accepted = 1) then 2 when id = $3 then 1 else 0 end as score from persons where actor->>'preferredUsername' = $4) where score > 0 order by score desc limit 1`, inReplyTo.AttributedTo, r.User.ID, fmt.Sprintf("https://%s/user/%s", cfg.Domain, mention[1]), mention[1]).Scan(&actorID)
		} else if mention[2] == "" && inReplyTo == nil {
			err = r.QueryRow(`select id from (select id, case when id = $1 then 2 when id in (select followed from follows where follower = $2 and accepted = 1) then 1 else 0 end as score from persons where actor->>'preferredUsername' = $3) where score > 0 order by score desc limit 1`, fmt.Sprintf("https://%s/user/%s", cfg.Domain, mention[1]), r.User.ID, mention[1]).Scan(&actorID)
		} else {
			err = r.QueryRow(`select id from persons where actor->>'preferredUsername' = $1 and id like $2`, mention[1], fmt.Sprintf("https://%s/%%", mention[2])).Scan(&actorID)
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
		tags = append(tags, ap.Mention{Type: ap.MentionMention, Name: mention[0], Href: actorID})
		cc.Add(actorID)
	}

	hash := sha256.Sum256([]byte(postID))

	note := ap.Object{
		Type:         ap.NoteObject,
		ID:           postID,
		AttributedTo: r.User.ID,
		Content:      plain.ToHTML(content),
		Published:    now,
		To:           to,
		CC:           cc,
		Tag:          tags,
	}

	if inReplyTo != nil {
		note.InReplyTo = inReplyTo.ID

		if inReplyTo.Type == ap.QuestionObject {
			options := inReplyTo.OneOf
			if len(options) == 0 {
				options = inReplyTo.AnyOf
			}

			for _, option := range options {
				if option.Name == note.Content {
					if inReplyTo.Closed != nil || inReplyTo.EndTime != nil && time.Now().After(*inReplyTo.EndTime) {
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

	if m := pollRegex.FindStringSubmatchIndex(note.Content); m != nil {
		optionNames := strings.SplitN(note.Content[m[4]:], pollOptionsDelimeter, pollMaxOptions+1)
		if len(optionNames) < pollMinOptions || len(optionNames) > pollMaxOptions {
			r.Log.Info("Received invalid poll", "content", note.Content)
			w.Statusf(40, "Polls must have %d to %d options", pollMinOptions, pollMaxOptions)
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

		note.Type = ap.QuestionObject
		note.Content = note.Content[m[2]:m[3]]
		endTime := time.Now().Add(pollDuration)
		note.EndTime = &endTime
	}

	if err := outbox.Create(r.Context, r.Log, r.DB, &note, r.User); err != nil {
		r.Log.Error("Failed to insert post", "error", err)
		if errors.Is(err, outbox.ErrDeliveryQueueFull) {
			w.Status(40, "Please try again later")
		} else {
			w.Error()
		}
		return
	}

	w.Redirectf("/users/view/%x", hash)
}
