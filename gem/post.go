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
	publicID     = activitypub.IRI("https://www.w3.org/ns/activitystreams#Public")
	mentionRegex = regexp.MustCompile(`\B@[^\s]+(@[^\s]+){0,1}`)
)

func init() {
	handlers[regexp.MustCompile(`^(/users/post$|/users/post\?.+)`)] = post
	handlers[regexp.MustCompile(`^(/users/public_post$|/users/public_post\?.+)`)] = publicPost
}

func publicPost(ctx context.Context, conn io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	postInternal(ctx, conn, requestUrl, params, user, db, nil, true)
}

func post(ctx context.Context, conn io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	postInternal(ctx, conn, requestUrl, params, user, db, nil, false)
}

func postInternal(ctx context.Context, conn io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB, inReplyTo *activitypub.Object, public bool) {
	if user == nil {
		conn.Write([]byte("30 /users\r\n"))
		return
	}

	if requestUrl.RawQuery == "" {
		if inReplyTo == nil {
			conn.Write([]byte("10 Post content\r\n"))
		} else {
			conn.Write([]byte("10 Reply content\r\n"))
		}
		return
	}

	content, err := url.QueryUnescape(requestUrl.RawQuery)
	if err != nil {
		conn.Write([]byte("40 Error\r\n"))
		conn.Write([]byte(err.Error()))
	}

	now := time.Now()

	id := fmt.Sprintf("https://%s/post/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", user.ID, content, now.Unix()))))

	audience := activitypub.ItemCollection{}
	if public && (inReplyTo == nil || !inReplyTo.To.Contains(publicID)) {
		audience.Append(publicID)
	} else {
		audience.Append(activitypub.IRI(fmt.Sprintf("https://%s/followers/%s", cfg.Domain, user.ID)))
	}

	parentAuthorID := ""
	if inReplyTo != nil {
		audience.Append(inReplyTo.AttributedTo.GetLink())
	}

	tags := activitypub.ItemCollection{}

	for _, mention := range mentionRegex.FindAllString(content, -1) {
		log.WithField("mention", mention).Info("Adding mention")
		id := activitypub.IRI(mention[1:])
		tags.Append(activitypub.MentionNew(id))
		audience.Append(id)
	}

	note := activitypub.Object{
		Type:         activitypub.NoteType,
		ID:           activitypub.IRI(id),
		AttributedTo: activitypub.LinkNew(activitypub.IRI(user.ID), activitypub.ActorType),
		Content:      activitypub.NaturalLanguageValuesNew(activitypub.LangRefValue{Ref: activitypub.NilLangRef, Value: []byte(content)}),
		Published:    now,
		To:           audience,
		Tag:          tags,
	}

	if inReplyTo != nil {
		note.InReplyTo = inReplyTo.ID
	}

	body, err := json.Marshal(note)
	if err != nil {
		fmt.Println(err)
		conn.Write([]byte("40 Error\r\n"))
		conn.Write([]byte(err.Error()))
		return
	}

	o := data.Object{
		ID:     id,
		Type:   "Note",
		Actor:  user.ID,
		Object: string(body),
	}

	if err := data.Objects.Insert(db, &o); err != nil {
		log.WithField("author", user.ID).Warn("Failed to insert post")
		conn.Write([]byte("40 Error\r\n"))
		conn.Write([]byte(err.Error()))
		return
	}

	followers, err := db.Query(`select distinct actor from objects where type = "Follow" and object = ?`, user.ID)
	if err != nil {
		log.WithFields(log.Fields{"author": user.ID, "post": o.ID}).Warn("Failed to list followers")
		conn.Write([]byte("40 Error\r\n"))
		return
	}
	defer followers.Close()

	create, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"type":     activitypub.CreateType,
		"id":       o.ID,
		"actor":    o.Actor,
		"object":   note,
	})
	if err != nil {
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	receivers := map[string]struct{}{}

	if parentAuthorID != "" {
		receivers[parentAuthorID] = struct{}{}
	}

	for _, tag := range note.Tag {
		tagObject, ok := tag.(*activitypub.Object)
		if !ok {
			continue
		}

		if tagObject.Type != activitypub.MentionType {
			continue
		}

		receivers[string(tagObject.ID.GetLink())] = struct{}{}
	}

	for followers.Next() {
		follower := ""
		if err := followers.Scan(&follower); err != nil {
			continue
		}

		receivers[follower] = struct{}{}
	}

	delete(receivers, user.ID)

	prefix := fmt.Sprintf("https://%s/user/", cfg.Domain)

	for receiver, _ := range receivers {
		if strings.HasPrefix(receiver, prefix) {
			continue
		}

		log.WithFields(log.Fields{"actor": user.ID, "receiver": receiver, "post": o.ID}).Info("Sending post")

		if err := fed.Send(ctx, db, user, receiver, string(create)); err != nil {
			log.WithFields(log.Fields{"actor": user.ID, "receiver": receiver, "post": o.ID}).WithError(err).Warn("Failed to send a post")
		}
	}

	conn.Write([]byte(fmt.Sprintf("30 /users/view?%s\r\n", url.QueryEscape(o.ID))))
}
