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
	"crypto/ed25519"
	"crypto/x509"
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
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
)

var (
	inboxRegex          = regexp.MustCompile(`^(did:key:z6Mk[a-km-zA-HJ-NP-Z1-9]+\/actor)\/inbox\?{0,1}.*`)
	portableObjectRegex = regexp.MustCompile(`^did:key:z6Mk[a-km-zA-HJ-NP-Z1-9]+\/[^#?]+`)
	followersRegex      = regexp.MustCompile(`^(did:key:z6Mk[a-km-zA-HJ-NP-Z1-9]+)\/actor\/followers$`)
	followersSyncRegex  = regexp.MustCompile(`^(did:key:z6Mk[a-km-zA-HJ-NP-Z1-9]+)\/actor\/followers_synchronization$`)
	iconRegex           = regexp.MustCompile(`^(did:key:z6Mk[a-km-zA-HJ-NP-Z1-9]+\/actor)\/icon\.` + icon.FileNameExtension[1:])
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

	var actor ap.Actor
	var rsaPrivKeyDer, ed25519PrivKey []byte
	if err := l.DB.QueryRowContext(r.Context(), `select json(actor), rsaprivkey, ed25519privkey from persons where cid = ? and ed25519privkey is not null`, receiver).Scan(&actor, &rsaPrivKeyDer, &ed25519PrivKey); errors.Is(err, sql.ErrNoRows) {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
	if err != nil {
		slog.Warn("Failed to parse RSA private key", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.doHandleInbox(w, r, [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
	})
}

func (l *Listener) fetchFollowersByHost(
	w http.ResponseWriter,
	r *http.Request,
	did string,
) (bool, *ap.Actor, string, *sql.Rows) {
	_, sender, err := l.verifyRequest(r, nil, ap.InstanceActor, l.AppActorKeys)
	if err != nil {
		slog.Warn("Failed to verify followers request", "did", did, "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return false, nil, "", nil
	}

	u, err := url.Parse(sender.ID)
	if err != nil {
		slog.Warn("Failed to extract sender host", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false, nil, "", nil
	}

	rows, err := l.DB.QueryContext(
		r.Context(),
		`SELECT follower FROM follows WHERE followed = 'https://' || ? || '/.well-known/apgateway/' || ? || '/actor' AND follower LIKE 'https://' || ? || '/%' AND accepted = 1`,
		l.Domain,
		did,
		u.Host,
	)
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false, nil, "", nil
	}

	return true, sender, fmt.Sprintf("https://%s/.well-known/apgateway/%s/actor/followers?domain=%s", l.Domain, did, u.Host), rows
}

func (l *Listener) fetchSenderFollowers(
	w http.ResponseWriter,
	r *http.Request,
	did string,
) (bool, *ap.Actor, string, *sql.Rows) {
	_, sender, err := l.verifyRequest(r, nil, 0, l.AppActorKeys)
	if err != nil {
		slog.Warn("Failed to verify followers request", "did", did, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return false, nil, "", nil
	}

	if origin, err := ap.Origin(sender.ID); err != nil {
		slog.Warn("Failed to extract sender origin", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return false, nil, "", nil
	} else if origin != did {
		slog.Warn("Denying followers request", "did", did, "sender", sender.ID)
		w.WriteHeader(http.StatusNotFound)
		return false, nil, "", nil
	}

	rows, err := l.DB.QueryContext(
		r.Context(),
		`SELECT follower FROM follows WHERE followed = 'https://' || ? || '/.well-known/apgateway/' || ? || '/actor' AND accepted = 1`,
		l.Domain,
		did,
	)
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false, nil, "", nil
	}

	return true, sender, fmt.Sprintf("https://%s/.well-known/apgateway/%s/actor/followers", l.Domain, did), rows
}

func (l *Listener) handleApGatewayFollowers(
	w http.ResponseWriter,
	r *http.Request,
	did string,
	fetch func(http.ResponseWriter, *http.Request, string) (bool, *ap.Actor, string, *sql.Rows),
) {
	ok, sender, collectionID, rows := fetch(w, r, did)
	if !ok {
		return
	}

	var items ap.Audience

	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		items.Add(follower)
	}
	defer rows.Close()

	if err := rows.Err(); err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	collection, err := json.Marshal(map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           collectionID,
		"type":         "OrderedCollection",
		"orderedItems": items,
	})
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("Received followers request", "did", did, "sender", sender.ID, "count", len(items.OrderedMap))

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write(collection)
}

func (l *Listener) handleAPGatewayGet(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	if m := followersSyncRegex.FindStringSubmatch(resource); m != nil {
		l.handleApGatewayFollowers(w, r, m[1], l.fetchFollowersByHost)
		return
	}

	if m := followersRegex.FindStringSubmatch(resource); m != nil {
		l.handleApGatewayFollowers(w, r, m[1], l.fetchSenderFollowers)
		return
	}

	if m := iconRegex.FindStringSubmatch(resource); m != nil {
		l.doHandleIcon(w, r, "ap://"+m[1])
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
