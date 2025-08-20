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
	inboxRegex    = regexp.MustCompile(`^(did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+)\/actor\/inbox[#?]{0,1}.*`)
	actorRegex    = regexp.MustCompile(`^(did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/actor)(?:\/z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+){0,1}[#?]{0,1}.*`)
	postRegex     = regexp.MustCompile(`^did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/actor\/post\/[^#?]{0,1}`)
	activityRegex = regexp.MustCompile(`^did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/[^#?]{0,1}`)
)

func (l *Listener) handleAPGatewayPost(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	m := inboxRegex.FindStringSubmatch(resource)
	if m == nil {
		slog.Info("Invalid resource", "resource", resource)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	receiver := m[1]

	var registered int
	if err := l.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where did = ? and ed25519privkey is not null)`, receiver).Scan(&registered); err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if registered == 0 {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	l.doHandleInbox(w, r)
}

func writeWithProof(w http.ResponseWriter, actor *ap.Actor, ed25519PrivKeyPem string, body []byte) {
	ed25519PrivKey, err := data.ParsePrivateKey(ed25519PrivKeyPem)
	if err != nil {
		slog.Warn("Failed to parse key", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	withProof, err := proof.Add(
		httpsig.Key{
			ID:         actor.AssertionMethod[0].ID,
			PrivateKey: ed25519PrivKey,
		},
		time.Now(),
		[]byte(body),
	)
	if err != nil {
		slog.Warn("Failed to add proof", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	w.Write(withProof)
}

func (l *Listener) getActor(w http.ResponseWriter, r *http.Request, id string) {
	slog.Info("Fetching actor", "id", id)

	var actor ap.Actor
	var actorString, ed25519PrivKeyPem string
	if err := l.DB.QueryRowContext(r.Context(), `select json(actor), json(actor), ed25519privkey from persons where (id = $1 or actor->>'$.assertionMethod[0].id' = $1) order by ed25519privkey is not null desc limit 1`, id).Scan(&actor, &actorString, &ed25519PrivKeyPem); errors.Is(err, sql.ErrNoRows) {
		slog.Info("Notifying about missing user", "id", id)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch user", "id", id, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if actor.Suspended {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	writeWithProof(w, &actor, ed25519PrivKeyPem, []byte(actorString))
}

func (l *Listener) getPost(w http.ResponseWriter, r *http.Request, id string) {
	slog.Info("Fetching post", "id", id)

	var note ap.Object
	var noteString, actorString, ed25519PrivKeyPem string
	var actor ap.Actor
	if err := l.DB.QueryRowContext(r.Context(), `select json(notes.object), json(notes.object), json(persons.actor), json(persons.actor), persons.ed25519privkey from notes join persons on persons.id = notes.author where notes.id = ? and persons.ed25519privkey is not null`, id).Scan(&note, &noteString, &actor, &actorString, &ed25519PrivKeyPem); errors.Is(err, sql.ErrNoRows) {
		slog.Info("Notifying about missing post", "id", id)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch post", "id", id, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !note.IsPublic() {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	writeWithProof(w, &actor, ed25519PrivKeyPem, []byte(noteString))
}

func (l *Listener) getActivity(w http.ResponseWriter, r *http.Request, id string) {
	slog.Info("Fetching activity", "id", id)

	var raw, ed25519PrivKeyPem string
	var activity ap.Activity
	var actor ap.Actor
	if err := l.DB.QueryRowContext(r.Context(), `select json(outbox.activity), json(outbox.activity), json(persons.actor), persons.ed25519privkey from outbox join persons on persons.id = outbox.activity->>'$.actor' where outbox.activity->>'$.id' = ?`, id).Scan(&raw, &activity, &actor, &ed25519PrivKeyPem); errors.Is(err, sql.ErrNoRows) {
		slog.Info("Notifying about missing activity", "id", id)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch activity", "id", id, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !activity.IsPublic() {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	writeWithProof(w, &actor, ed25519PrivKeyPem, []byte(raw))
}

func (l *Listener) handleAPGatewayGet(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	if m := actorRegex.FindStringSubmatch(resource); m != nil {
		l.getActor(w, r, ap.Gateway("https://"+l.Domain, "ap://"+m[1]))
	} else if m := postRegex.FindStringSubmatch(resource); m != nil {
		l.getPost(w, r, ap.Gateway("https://"+l.Domain, "ap://"+m[1]))
	} else if m := activityRegex.FindStringSubmatch(resource); m != nil {
		l.getActivity(w, r, ap.Gateway("https://"+l.Domain, "ap://"+m[1]))
	} else {
		slog.Info("Invalid resource", "resource", resource)
		w.WriteHeader(http.StatusNotFound)
	}
}
