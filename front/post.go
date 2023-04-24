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
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/text"
	log "github.com/sirupsen/logrus"
	"net/url"
	"regexp"
	"time"
)

var (
	mentionRegex = regexp.MustCompile(`\B@([a-zA-Z0-9]+)(@[a-z0-9.]+){0,1}`)
	hashtagRegex = regexp.MustCompile(`(\B#[^\s]{1,32})`)
)

func post(w text.Writer, r *request, inReplyTo *ap.Object, to ap.Audience, cc ap.Audience, prompt string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := time.Now()

	var today, last sql.NullInt64
	if err := r.QueryRow(`select count(*), max(inserted) from notes where author = ? and inserted > ?`, r.User.ID, now.Add(-24*time.Hour).Unix()).Scan(&today, &last); err != nil {
		r.Log.WithError(err).Warn("Failed to check if new post needs to be throttled")
		w.Error()
		return
	}

	if today.Valid && today.Int64 >= 30 {
		r.Log.WithField("posts", today.Int64).Warn("User has exceeded the daily posts quota")
		w.Status(40, "Please wait before posting again")
		return
	}

	if today.Valid && last.Valid {
		t := time.Unix(last.Int64, 0)
		interval := time.Duration(today.Int64/2) * time.Minute
		if now.Sub(t) < interval {
			r.Log.WithFields(log.Fields{"last": t, "can": t.Add(interval)}).Warn("User is posting too frequently")
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

	postID := fmt.Sprintf("https://%s/post/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", r.User.ID, content, now.Unix()))))

	tags := ap.Mentions{}

	for _, hashtag := range hashtagRegex.FindAllString(content, -1) {
		tags = append(tags, ap.Mention{Type: ap.HashtagMention, Name: hashtag})
	}

	for _, mention := range mentionRegex.FindAllStringSubmatch(content, -1) {
		if len(mention) < 3 {
			continue
		}
		var actorID string
		var err error
		if mention[2] == "" && inReplyTo != nil {
			err = r.QueryRow(`select id from persons where id = ? or (id in (select followed from follows where follower = ?) and actor->>'preferredUsername' = ?) or id = ?`, inReplyTo.AttributedTo, r.User.ID, mention[1], fmt.Sprintf("https://%s/user/%s", cfg.Domain, mention[1])).Scan(&actorID)
		} else if mention[2] == "" && inReplyTo == nil {
			err = r.QueryRow(`select id from persons where (id in (select followed from follows where follower = ?) and actor->>'preferredUsername' = ?) or id = ?`, r.User.ID, mention[1], fmt.Sprintf("https://%s/user/%s", cfg.Domain, mention[1])).Scan(&actorID)
		} else {
			err = r.QueryRow(`select id from persons where id like ? and actor->>'preferredUsername' = ?`, fmt.Sprintf("https://%s/%%", mention[2][1:]), mention[1]).Scan(&actorID)
		}

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				r.Log.WithField("mention", mention[0]).Warn("Failed to guess mentioned actor ID")
			} else {
				r.Log.WithField("mention", mention[0]).WithError(err).Warn("Failed to guess mentioned actor ID")
			}
			continue
		}

		r.Log.WithFields(log.Fields{"name": mention[0], "actor": actorID}).Info("Adding mention")
		tags = append(tags, ap.Mention{Type: ap.MentionMention, Name: mention[0], Href: actorID})
		to.Add(actorID)
	}

	note := ap.Object{
		Type:         ap.NoteObject,
		ID:           postID,
		AttributedTo: r.User.ID,
		Content:      content,
		Published:    now,
		To:           to,
		CC:           cc,
		Tag:          tags,
	}

	if inReplyTo != nil {
		note.InReplyTo = inReplyTo.ID
	}

	if err := fed.Deliver(r.Context, r.DB, r.Log, &note, r.User); err != nil {
		r.Log.WithField("author", r.User.ID).Error("Failed to insert post")
		if errors.Is(err, fed.DeliveryQueueFull) {
			w.Status(40, "Please try again later")
		} else {
			w.Error()
		}
		return
	}

	w.Redirectf("/users/view/%x", sha256.Sum256([]byte(postID)))
}
