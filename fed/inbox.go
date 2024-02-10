/*
Copyright 2023, 2024 Dima Krasner

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
	"bytes"
	"encoding/json"
	"errors"
	"github.com/dimkr/tootik/ap"
	"io"
	"net/http"
)

func (l *Listener) handleInbox(w http.ResponseWriter, r *http.Request) {
	receiver := r.PathValue("username")

	var registered int
	if err := l.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where actor->>'preferredUsername' = ? and host = ?)`, receiver, l.Domain).Scan(&registered); err != nil {
		l.Log.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if registered == 0 {
		l.Log.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, l.Config.MaxRequestBodySize))
	if err != nil {
		return
	}

	var activity ap.Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		l.Log.Warn("Failed to unmarshal activity", "body", string(body), "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	// if actor is deleted, ignore this activity if we don't know this actor
	offline := false
	if activity.Type == ap.DeleteActivity {
		offline = true
	}

	sender, err := verify(r.Context(), l.Domain, l.Log, r, l.DB, l.Resolver, l.Actor, offline)
	if err != nil {
		if errors.Is(err, ErrActorGone) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrActorNotCached) {
			l.Log.Debug("Ignoring Delete activity for unknown actor", "error", err)
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrBlockedDomain) {
			l.Log.Debug("Failed to verify activity", "activity", activity.ID, "type", activity.Type, "error", err)
		} else {
			l.Log.Warn("Failed to verify activity", "activity", activity.ID, "type", activity.Type, "error", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if _, err = l.DB.ExecContext(
		r.Context(),
		`INSERT OR IGNORE INTO inbox (sender, activity) VALUES(?,?)`,
		sender.ID,
		string(body),
	); err != nil {
		l.Log.Error("Failed to insert activity", "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
