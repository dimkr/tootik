/*
Copyright 2023 Dima Krasner

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
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
)

const activitiesPerPage = 30

type outboxHandler struct {
	Log *slog.Logger
	DB  *sql.DB
}

func (h *outboxHandler) getCollection(w http.ResponseWriter, r *http.Request, username, actorID string) {
	collection := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       fmt.Sprintf("https://%s/outbox/%s", cfg.Domain, username),
		"type":     "OrderedCollection",
	}

	h.Log.Info("Listing activities by user", "username", username)

	var totalItems sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context(), `select count(*) from notes join outbox on outbox.activity->>'object.id' = notes.id where notes.author = ? and notes.public = 1`, actorID).Scan(&totalItems); err != nil {
		h.Log.Warn("Failed to count activities", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if totalItems.Valid {
		collection["totalItems"] = totalItems.Int64
	} else {
		collection["totalItems"] = 0
	}

	var firstSince sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context(), `select min(outbox.inserted) from notes join outbox on outbox.activity->>'object.id' = notes.id where notes.author = ? and notes.public = 1`, actorID, activitiesPerPage).Scan(&firstSince); err != nil {
		h.Log.Warn("Failed to get first page timestamp", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if firstSince.Valid {
		collection["first"] = fmt.Sprintf("https://%s/outbox/%s?0%d", cfg.Domain, username, firstSince.Int64)
	}

	if totalItems.Valid && totalItems.Int64 > activitiesPerPage {
		var lastSince sql.NullInt64
		if err := h.DB.QueryRowContext(r.Context(), `select min(inserted) from (select outbox.inserted from notes join outbox on outbox.activity->>'object.id' = notes.id where notes.author = ? and notes.public = 1 order by outbox.inserted desc limit ?)`, actorID, activitiesPerPage).Scan(&lastSince); err != nil {
			h.Log.Warn("Failed to get last page timestamp", "username", username, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if lastSince.Valid {
			collection["last"] = fmt.Sprintf("https://%s/outbox/%s?%d", cfg.Domain, username, lastSince.Int64)
		}
	} else if firstSince.Valid {
		collection["last"] = collection["first"]
	}

	j, err := json.Marshal(collection)
	if err != nil {
		h.Log.Warn("Failed to marshal collection", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	w.Write(j)
}

func (h *outboxHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	username := filepath.Base(r.URL.Path)
	actorID := fmt.Sprintf("https://%s/user/%s", cfg.Domain, username)

	if shouldRedirect(r) {
		outbox := fmt.Sprintf("gemini://%s/outbox/%x", cfg.Domain, sha256.Sum256([]byte(actorID)))
		h.Log.Info("Redirecting to outbox over Gemini", "outbox", outbox)
		w.Header().Set("Location", outbox)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	h.Log.Info("Fetching activities by user", "username", username)

	var exists int
	if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where id = ?)`, actorID).Scan(&exists); err != nil {
		h.Log.Warn("Failed to check if user exists", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.URL.RawQuery == "" {
		h.getCollection(w, r, username, actorID)
		return
	}

	since, err := strconv.ParseInt(r.URL.RawQuery, 10, 64)
	if err != nil {
		h.Log.Warn("Failed to parse offset", "username", username, "query", r.URL.RawQuery, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if since < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rows, err := h.DB.QueryContext(r.Context(), `select outbox.activity from notes join outbox on outbox.activity->>'object.id' = notes.id where notes.author = $1 and notes.public = 1 and outbox.inserted >= $2 order by notes.inserted limit $3`, actorID, since, activitiesPerPage)
	if err != nil {
		h.Log.Warn("Failed to fetch activities", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	activities := make([]ap.Activity, 0, activitiesPerPage)

	for rows.Next() {
		var activityString string
		if err := rows.Scan(&activityString); err != nil {
			h.Log.Warn("Failed to scan activity", "error", err)
			continue
		}

		var activity ap.Activity
		if err := json.Unmarshal([]byte(activityString), &activity); err != nil {
			h.Log.Warn("Failed to unmarshal activity", "error", err)
			continue
		}

		activity.Context = nil
		activities = append(activities, activity)
	}
	rows.Close()

	page := map[string]any{
		"@context":     []string{"https://www.w3.org/ns/activitystreams"},
		"id":           fmt.Sprintf("https://%s/outbox/%s?%d", cfg.Domain, username, since),
		"type":         "OrderedCollectionPage",
		"partOf":       fmt.Sprintf("https://%s/outbox/%s", cfg.Domain, username),
		"orderedItems": activities,
	}

	var nextSince sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context(), `select max(inserted) from (select outbox.inserted from notes join outbox on outbox.activity->>'object.id' = notes.id where notes.author = $1 and notes.public = 1 and outbox.inserted > $2 order by outbox.inserted limit $3 offset $3)`, actorID, since, activitiesPerPage).Scan(&nextSince); err != nil {
		h.Log.Warn("Failed to get next page timestamp", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if nextSince.Valid {
		page["next"] = fmt.Sprintf("https://%s/outbox/%s?%d", cfg.Domain, username, nextSince.Int64)
	}

	var prevSince sql.NullInt64
	if err := h.DB.QueryRowContext(r.Context(), `select min(inserted) from (select outbox.inserted from notes join outbox on outbox.activity->>'object.id' = notes.id where notes.author = ? and notes.public = 1 and outbox.inserted < ? order by outbox.inserted desc limit ?)`, actorID, since, activitiesPerPage).Scan(&prevSince); err != nil {
		h.Log.Warn("Failed to get previous page timestamp", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if prevSince.Valid {
		page["prev"] = fmt.Sprintf("https://%s/outbox/%s?%d", cfg.Domain, username, prevSince.Int64)
	}

	j, err := json.Marshal(page)
	if err != nil {
		h.Log.Warn("Failed to marshal page", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	w.Write(j)
}
