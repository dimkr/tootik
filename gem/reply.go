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
	"database/sql"
	"encoding/json"
	"github.com/dimkr/tootik/data"
	"github.com/go-ap/activitypub"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/reply/[a-f0-9]+`)] = reply
}

func reply(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	hash := filepath.Base(requestUrl.Path)

	if hash == "" {
		log.Warn("Received reply request without post hash")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	post, err := data.Objects.GetByHash(hash, db)
	if err != nil {
		log.WithField("hash", hash).WithError(err).Warn("Failed to find post by hash")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	note := activitypub.Object{}
	if err := json.Unmarshal([]byte(post.Object), &note); err != nil {
		w.Write([]byte("40 Error\r\n"))
		log.WithField("post", string(note.ID.GetLink())).WithError(err).Warn("Failed to unmarshal post")
		return
	}

	log.WithField("post", string(note.ID.GetLink())).Info("Replying to post")

	postInternal(ctx, w, requestUrl, params, user, db, &note, false)
}
