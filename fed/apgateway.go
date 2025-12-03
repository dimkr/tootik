/*
Copyright 2025 Dima Krasner

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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/danger"
)

var (
	inboxRegex          = regexp.MustCompile(`^(did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/actor)\/inbox\?{0,1}.*`)
	portableObjectRegex = regexp.MustCompile(`^did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/[^#?]+`)
	followersRegex      = regexp.MustCompile(`^(did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+)\/actor\/followers_synchronization`)
)

func (l *Listener) handleAPGatewayPost(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	m := inboxRegex.FindStringSubmatch(resource)
	if m == nil {
		slog.Info("Invalid resource", "resource", resource)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	receiver := "ap://" + m[1]

	var exists int
	if err := l.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where cid = ? and ed25519privkey is not null)`, receiver).Scan(&exists); err == nil && exists == 0 {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.doHandleInbox(w, r)
}

func (l *Listener) handleApGatewayFollowers(w http.ResponseWriter, r *http.Request, did string) {
	_, sender, err := l.verifyRequest(r, nil, ap.InstanceActor)
	if err != nil {
		slog.Warn("Failed to verify followers request", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	u, err := url.Parse(sender.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := l.DB.QueryContext(r.Context(), `SELECT follower FROM follows WHERE followed = 'https://' || ? || '/.well-known/apgateway/' || ? || '/actor' AND follower LIKE 'https://' || ? || '/%' AND accepted = 1`, l.Domain, did, u.Host)
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "host", u.Host, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var items ap.Audience

	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			slog.Warn("Failed to fetch followers", "did", did, "host", u.Host, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		items.Add(follower)
	}
	defer rows.Close()

	if err := rows.Err(); err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "host", u.Host, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	collection, err := json.Marshal(map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           fmt.Sprintf("https://%s/.well-known/apgateway/%s/actor/followers?domain=%s", l.Domain, did, u.Host),
		"type":         "OrderedCollection",
		"orderedItems": items,
	})
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "host", u.Host, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("Received followers request", "sender", sender.ID, "did", did, "host", u.Host, "count", len(items.OrderedMap))

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write(collection)
}

func (l *Listener) handleAPGatewayGet(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	if m := followersRegex.FindStringSubmatch(resource); m != nil {
		l.handleApGatewayFollowers(w, r, m[1])
		return
	}

	id := portableObjectRegex.FindString(resource)
	if id == "" {
		slog.Info("Invalid resource", "resource", resource)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	id = "ap://" + id

	slog.Info("Fetching object", "id", id)

	var raw string
	if err := l.DB.QueryRowContext(
		r.Context(),
		`
		select raw from
		(
			select json(actor) as raw from persons
			where cid = $1 and ed25519privkey is not null
			union all
			select json(notes.object) as raw from notes
			join persons on notes.author = persons.id
			where notes.cid = $1 and notes.public = 1 and persons.ed25519privkey is not null
			union all
			select json(outbox.activity) as raw from outbox
			join persons on outbox.activity->>'$.actor' = persons.id
			where outbox.cid = $1 and (exists (select 1 from json_each(outbox.activity->'$.cc') where value = $2) or exists (select 1 from json_each(outbox.activity->'$.to') where value = $2)) and persons.ed25519privkey is not null
		)
		limit 1
		`,
		id,
		ap.Public,
	).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		slog.Info("Notifying about missing object", "id", id)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch object", "id", id, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	w.Write(danger.Bytes(raw))
}
