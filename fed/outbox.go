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
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

func (l *Listener) getCollection(w http.ResponseWriter, username string) {
	first := fmt.Sprintf("https://%s/outbox/%s?0", l.Domain, username)

	collection := map[string]any{
		"@context":   "https://www.w3.org/ns/activitystreams",
		"id":         fmt.Sprintf("https://%s/outbox/%s", l.Domain, username),
		"type":       "OrderedCollection",
		"first":      first,
		"last":       first,
		"totalItems": 0,
	}

	slog.Info("Listing activities by user", "username", username)

	j, err := json.Marshal(collection)
	if err != nil {
		slog.Warn("Failed to marshal collection", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	w.Write(j)
}

func (l *Listener) handleOutbox(w http.ResponseWriter, r *http.Request) {
	username := r.PathValue("username")

	var actorID sql.NullString
	if err := l.DB.QueryRowContext(r.Context(), `select id from persons where actor->>'$.preferredUsername' = ? and host = ?`, username, l.Domain).Scan(&actorID); err != nil {
		slog.Warn("Failed to check if user exists", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !actorID.Valid {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if shouldRedirect(r) {
		outbox := fmt.Sprintf("gemini://%s/outbox/%s", l.Domain, strings.TrimPrefix(actorID.String, "https://"))
		slog.Info("Redirecting to outbox over Gemini", "outbox", outbox)
		w.Header().Set("Location", outbox)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	slog.Info("Fetching activities by user", "username", username)

	if r.URL.RawQuery == "" {
		l.getCollection(w, username)
		return
	}

	since, err := strconv.ParseInt(r.URL.RawQuery, 10, 64)
	if err != nil {
		slog.Warn("Failed to parse offset", "username", username, "query", r.URL.RawQuery, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if since < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	first := fmt.Sprintf("https://%s/outbox/%s?0", l.Domain, username)

	page := map[string]any{
		"@context":     []string{"https://www.w3.org/ns/activitystreams"},
		"id":           fmt.Sprintf("https://%s/outbox/%s?%d", l.Domain, username, since),
		"type":         "OrderedCollectionPage",
		"partOf":       fmt.Sprintf("https://%s/outbox/%s", l.Domain, username),
		"orderedItems": []ap.Activity{},
		"next":         first,
		"prev":         first,
	}

	j, err := json.Marshal(page)
	if err != nil {
		slog.Warn("Failed to marshal page", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	w.Write(j)
}
