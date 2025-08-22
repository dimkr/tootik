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
	inboxRegex = regexp.MustCompile(`^(did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/actor)\/inbox[#?]{0,1}.*`)
	actorRegex = regexp.MustCompile(`^(did:key:z6Mk[123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz]+\/actor)[#?]{0,1}.*`)
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

	var id string
	if err := l.DB.QueryRowContext(r.Context(), `select id from persons where cid = ? and ed25519privkey is not null`, receiver).Scan(&id); errors.Is(err, sql.ErrNoRows) {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.doHandleInbox(w, r, id)
}

func (l *Listener) getActor(w http.ResponseWriter, r *http.Request, id string) {
	slog.Info("Fetching actor", "id", id)

	var actor ap.Actor
	var actorString, ed25519PrivKeyMultibase string
	if err := l.DB.QueryRowContext(r.Context(), `select json(actor), json(actor), ed25519privkey from persons where cid = ? order by ed25519privkey is not null desc limit 1`, id).Scan(&actor, &actorString, &ed25519PrivKeyMultibase); errors.Is(err, sql.ErrNoRows) {
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

	ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
	if err != nil {
		slog.Warn("Failed to decode key", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	withProof, err := proof.Add(
		httpsig.Key{
			ID:         actor.AssertionMethod[0].ID,
			PrivateKey: ed25519PrivKey,
		},
		time.Now(),
		[]byte(actorString),
	)
	if err != nil {
		slog.Warn("Failed to add proof", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	w.Write(withProof)
}

func (l *Listener) handleAPGatewayGet(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	if m := actorRegex.FindStringSubmatch(resource); m != nil {
		l.getActor(w, r, "ap://"+m[1])
	} else {
		slog.Info("Invalid resource", "resource", resource)
		w.WriteHeader(http.StatusNotFound)
	}
}
