/*
Copyright 2024 Dima Krasner

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

package fed

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"net/http"
	"net/url"
)

type partialFollowers map[string]string

func digestFollowers(ctx context.Context, db *sql.DB, actorID, host string) ([sha256.Size]byte, error) {
	var digest [sha256.Size]byte

	rows, err := db.QueryContext(ctx, `select follower from follows where followed = ? and follower like 'https://' || ? || '/' || '%' and accepted = 1`, actorID, host)
	if err != nil {
		return digest, err
	}
	defer rows.Close()

	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			return digest, err
		}
		hash := sha256.Sum256([]byte(follower))
		for i := range sha256.Size {
			digest[i] ^= hash[i]
		}
	}

	return digest, nil
}

func (f partialFollowers) Digest(ctx context.Context, db *sql.DB, domain string, actor *ap.Actor, req *http.Request) error {
	if header, ok := f[req.URL.Host]; ok && header != "" {
		req.Header.Set("Collection-Synchronization", header)
		return nil
	}

	digest, err := digestFollowers(ctx, db, actor.ID, req.URL.Host)
	if err != nil {
		return err
	}

	header := fmt.Sprintf(`collectionId="%s", url="https://%s/followers_synchronization/%s", digest="%x"`, actor.Followers, domain, actor.PreferredUsername, digest)
	f[req.URL.Host] = header
	req.Header.Set("Collection-Synchronization", header)
	return nil
}

func (l *Listener) handleFollowers(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")

	sender, err := verify(r.Context(), l.Domain, l.Log, r, l.DB, l.Resolver, l.Actor, true)
	if err != nil {
		l.Log.Warn("Failed to verify followers request", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	u, err := url.Parse(sender.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := l.DB.QueryContext(r.Context(), `select follower from follows where followed = 'https://' || ? || '/user/' || ? and follower like 'https://' || ? || '/' || '%'`, l.Domain, name, u.Host)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var items ap.Audience

	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			rows.Close()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		items.Add(follower)
	}
	rows.Close()

	collection, err := json.Marshal(map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           fmt.Sprintf("https://%s/followers/%s?domain=%s", l.Domain, name, u.Host),
		"type":         "OrderedCollection",
		"orderedItems": items,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.Log.Info("Received followers request", "username", name, "host", u.Host, "response", collection)

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write([]byte(collection))
}
