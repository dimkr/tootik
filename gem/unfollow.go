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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^(/users/unfollow$|/users/unfollow\?.+)`)] = withUserMenu(unfollow)
}

func unfollow(ctx context.Context, conn io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
	if user == nil {
		conn.Write([]byte("30 /users\r\n"))
		return
	}

	if requestUrl.RawQuery == "" {
		conn.Write([]byte("10 User to unfollow\r\n"))
		return
	}

	followed, err := url.QueryUnescape(requestUrl.RawQuery)
	if err != nil {
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	if followed == user.ID {
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	var followID string
	if err := db.QueryRow(`select id from objects where type = 'Follow' and actor = ? and object = ?`, user.ID, followed).Scan(&followID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithFields(log.Fields{"follower": user.ID, "followed": followed}).WithError(err).Warn("Failed to remove Follow")
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	id := fmt.Sprintf("https://%s/undo/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s", user.ID, followed))))

	body, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       id,
		"type":     "Follow",
		"actor":    user.ID,
		"object": map[string]any{
			"id":     followID,
			"type":   "Follow",
			"actor":  user.ID,
			"object": followed,
		},
	})
	if err != nil {
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	if err := fed.Send(ctx, db, user, followed, string(body)); err != nil {
		log.WithFields(log.Fields{"follower": user.ID, "followed": followed, "id": followID}).WithError(err).Warn("Failed to request Undo")
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	if _, err := db.Exec(`delete from objects where id = ?`, followID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.WithFields(log.Fields{"follower": user.ID, "followed": followed, "id": followID}).WithError(err).Warn("Failed to remove Follow")
		conn.Write([]byte("40 Error\r\n"))
		return
	}

	following(ctx, conn, requestUrl, params, user, db)
}
