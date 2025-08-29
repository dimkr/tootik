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
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

var (
	inboxRegex          = regexp.MustCompile(`^(did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/actor)\/inbox[#?]{0,1}.*`)
	portableObjectRegex = regexp.MustCompile(`^did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/[^#?]+`)
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
	var rsaPrivKeyPem, ed25519PrivKeyMultibase string
	if err := l.DB.QueryRowContext(r.Context(), `select json(actor), rsaprivkey, ed25519privkey from persons where cid = ? and ed25519privkey is not null`, receiver).Scan(&actor, &rsaPrivKeyPem, &ed25519PrivKeyMultibase); errors.Is(err, sql.ErrNoRows) {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rsaPrivKey, err := data.ParseRSAPrivateKey(rsaPrivKeyPem)
	if err != nil {
		slog.Warn("Failed to parse RSA private key", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
	if err != nil {
		slog.Warn("Failed to decode Ed25519 private key", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.doHandleInbox(w, r, actor.ID, [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey},
	})
}

func (l *Listener) handleAPGatewayGet(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	id := portableObjectRegex.FindString(resource)
	if id == "" {
		slog.Info("Invalid resource", "resource", resource)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	id = "ap://" + id

	slog.Info("Fetching object", "id", id)

	var actor ap.Actor
	var raw, ed25519PrivKeyMultibase string
	if err := l.DB.QueryRowContext(
		r.Context(),
		`
		select actor, ed25519privkey, raw from
		(
			select json(actor) as actor, ed25519privkey, json(actor) as raw from persons
			where cid = $1 and ed25519privkey is not null
			union all
			select json(persons.actor) as actor, persons.ed25519privkey, json(notes.object) as raw from notes
			join persons on notes.author = persons.id
			where notes.cid = $1 and notes.public = 1 and persons.ed25519privkey is not null
			union all
			select json(persons.actor) as actor, persons.ed25519privkey, json(outbox.activity) as raw from outbox
			join persons on outbox.activity->>'$.actor' = persons.id
			where outbox.cid = $1 and (exists (select 1 from json_each(outbox.activity->'$.cc') where value = $2) or exists (select 1 from json_each(outbox.activity->'$.to') where value = $2)) and persons.ed25519privkey is not null
		)
		limit 1
		`,
		id,
		ap.Public,
	).Scan(&actor, &ed25519PrivKeyMultibase, &raw); errors.Is(err, sql.ErrNoRows) {
		slog.Info("Notifying about missing object", "id", id)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch object", "id", id, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
	if err != nil {
		slog.Warn("Failed to decode key", "id", id, "key", actor.AssertionMethod[0].ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	withProof, err := proof.Add(
		httpsig.Key{
			ID:         actor.AssertionMethod[0].ID,
			PrivateKey: ed25519PrivKey,
		},
		time.Now(),
		[]byte(raw),
	)
	if err != nil {
		slog.Warn("Failed to add proof", "id", id, "key", actor.AssertionMethod[0].ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	w.Write(withProof)
}
