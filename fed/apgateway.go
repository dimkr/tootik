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
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func (l *Listener) handleAPGateway(w http.ResponseWriter, r *http.Request) {
	resource := r.PathValue("resource")

	if r.Method == http.MethodPost && strings.Contains(resource, "/actor/inbox") {
		inbox := "ap://" + resource

		slog.Info("Posting to inbox", "inbox", inbox)

		var exists int
		if err := l.DB.QueryRowContext(r.Context(), `select 1 from persons where actor->>'$.inbox' = ? and ed25519privkey is not null`, inbox).Scan(&exists); err != nil {
			slog.Warn("Failed to check if inbox exists", "inbox", inbox, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if err != nil {
			slog.Info("Notifying about missing user", "inbox", inbox)
			w.WriteHeader(http.StatusNotFound)
			return
		}

		l.doHandleInbox(w, r)
		return
	}

	if r.Method == http.MethodGet && strings.Contains(resource, "/actor") {
		id := "ap://" + resource

		slog.Info("Fetching actor", "id", id)

		var actor ap.Actor
		var actorString, ed25519PrivKeyPem string
		if err := l.DB.QueryRowContext(r.Context(), `select json(actor), json(actor), ed25519privkey from persons where (id = $1 or actor->>'$.assertionMethod[0].id' = $1) and ed25519privkey is not null and actor->>'$.assertionMethod[0].id' is not null`, id).Scan(&actor, &actorString, &ed25519PrivKeyPem); errors.Is(err, sql.ErrNoRows) {
			slog.Info("Notifying about missing user", "id", id)
			w.WriteHeader(http.StatusNotFound)
			return
		} else if err != nil {
			slog.Warn("Failed to fetch resource", "id", id, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		ed25519PrivKey, err := data.ParsePrivateKey(ed25519PrivKeyPem)
		if err != nil {
			slog.Warn("Failed to parse key", "id", id, "error", err)
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
			slog.Warn("Failed to add proof", "id", id, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
		w.Write(withProof)
		return
	}

	w.WriteHeader(http.StatusNotFound)

	// TODO: 405
}
