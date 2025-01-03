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
	"log/slog"
	"net/http"
)

func (l *Listener) handleInbox(w http.ResponseWriter, r *http.Request) {
	receiver := r.PathValue("username")

	var registered int
	if err := l.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where actor->>'$.preferredUsername' = ? and host = ?)`, receiver, l.Domain).Scan(&registered); err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if registered == 0 {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.ContentLength > l.Config.MaxRequestBodySize {
		slog.Warn("Ignoring big request", "size", r.ContentLength)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, l.Config.MaxRequestBodySize))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var activity ap.Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		slog.Warn("Failed to unmarshal activity", "body", string(body), "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(body))

	// if actor is deleted, ignore this activity if we don't know this actor
	var flags ap.ResolverFlag
	if activity.Type == ap.Delete {
		flags |= ap.Offline
	}

	sender, err := l.verify(r, body, flags)
	if err != nil {
		if errors.Is(err, ErrActorGone) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrActorNotCached) {
			slog.Debug("Ignoring Delete activity for unknown actor", "error", err)
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrBlockedDomain) {
			slog.Debug("Failed to verify activity", "activity", activity.ID, "type", activity.Type, "error", err)
		} else {
			slog.Warn("Failed to verify activity", "activity", activity.ID, "type", activity.Type, "error", err)
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
		slog.Error("Failed to insert activity", "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	followersSync := r.Header.Get("Collection-Synchronization")
	if followersSync != "" {
		if err := l.saveFollowersDigest(r.Context(), sender, followersSync); err != nil {
			slog.Warn("Failed to save followers sync header", "sender", sender.ID, "header", followersSync, "error", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}
