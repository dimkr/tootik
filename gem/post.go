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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
	"time"
)

const minPostInterval = time.Minute * 5

var mentionRegex = regexp.MustCompile(`\B@([a-zA-Z0-9]+)(@[a-z0-9.]+){0,1}`)

func post(w io.Writer, r *request, inReplyTo *ap.Object, to ap.Audience, cc ap.Audience) {
	if r.User == nil {
		w.Write([]byte("30 /users\r\n"))
		return
	}

	var throttle int
	if err := r.QueryRow(`select exists (select 1 from notes where author = ? and inserted > ?)`, r.User.ID, time.Now().Add(-minPostInterval)).Scan(&throttle); err != nil {
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if throttle == 1 {
		r.Log.Warn("User is posting too frequently")
		w.Write([]byte("40 Please wait before posting again\r\n"))
		return
	}

	if r.URL.RawQuery == "" {
		if inReplyTo == nil {
			w.Write([]byte("10 Post content\r\n"))
		} else {
			w.Write([]byte("10 Reply content\r\n"))
		}
		return
	}

	content, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		w.Write([]byte("40 Error\r\n"))
		return
	}

	now := time.Now()
	postID := fmt.Sprintf("https://%s/post/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", r.User.ID, content, now.Unix()))))

	tags := ap.Mentions{}

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
			w.Write([]byte("40 Please try again later\r\n"))
		} else {
			w.Write([]byte("40 Error\r\n"))
		}
		return
	}

	w.Write([]byte(fmt.Sprintf("30 /users/view/%x\r\n", sha256.Sum256([]byte(postID)))))
}
