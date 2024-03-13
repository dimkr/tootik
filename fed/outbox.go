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
	"net/http"
	"strconv"
	"strings"
)

const activitiesPerPage = 30

func (l *Listener) getCollection(w http.ResponseWriter, r *http.Request, username, actorID string) {
	collection := map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       fmt.Sprintf("https://%s/outbox/%s", l.Domain, username),
		"type":     "OrderedCollection",
	}

	l.Log.Info("Listing activities by user", "username", username)

	var totalItems sql.NullInt64
	if err := l.DB.QueryRowContext(r.Context(), `select count(*) from notes join outbox on outbox.activity->>'$.object.id' = notes.id where outbox.sender = $1 and notes.author = $1 and notes.public = 1`, actorID).Scan(&totalItems); err != nil {
		l.Log.Warn("Failed to count activities", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if totalItems.Valid {
		collection["totalItems"] = totalItems.Int64
	} else {
		collection["totalItems"] = 0
	}

	var firstSince sql.NullInt64
	if err := l.DB.QueryRowContext(r.Context(), `select min(outbox.inserted) from notes join outbox on outbox.activity->>'$.object.id' = notes.id where outbox.sender = $1 and notes.author = $1 and notes.public = 1`, actorID, activitiesPerPage).Scan(&firstSince); err != nil {
		l.Log.Warn("Failed to get first page timestamp", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if firstSince.Valid {
		collection["first"] = fmt.Sprintf("https://%s/outbox/%s?0%d", l.Domain, username, firstSince.Int64)
	}

	if totalItems.Valid && totalItems.Int64 > activitiesPerPage {
		var lastSince sql.NullInt64
		if err := l.DB.QueryRowContext(r.Context(), `select min(inserted) from (select outbox.inserted from notes join outbox on outbox.activity->>'$.object.id' = notes.id where outbox.sender = $1 and notes.author = $1 and notes.public = 1 order by outbox.inserted desc limit $2)`, actorID, activitiesPerPage).Scan(&lastSince); err != nil {
			l.Log.Warn("Failed to get last page timestamp", "username", username, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if lastSince.Valid {
			collection["last"] = fmt.Sprintf("https://%s/outbox/%s?%d", l.Domain, username, lastSince.Int64)
		}
	} else if firstSince.Valid {
		collection["last"] = collection["first"]
	}

	j, err := json.Marshal(collection)
	if err != nil {
		l.Log.Warn("Failed to marshal collection", "username", username, "error", err)
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
		l.Log.Warn("Failed to check if user exists", "username", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !actorID.Valid {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if shouldRedirect(r) {
		outbox := fmt.Sprintf("gemini://%s/outbox/%s", l.Domain, strings.TrimPrefix(actorID.String, "https://"))
		l.Log.Info("Redirecting to outbox over Gemini", "outbox", outbox)
		w.Header().Set("Location", outbox)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	l.Log.Info("Fetching activities by user", "username", username)

	if r.URL.RawQuery == "" {
		l.getCollection(w, r, username, actorID.String)
		return
	}

	since, err := strconv.ParseInt(r.URL.RawQuery, 10, 64)
	if err != nil {
		l.Log.Warn("Failed to parse offset", "username", username, "query", r.URL.RawQuery, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if since < 0 {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	rows, err := l.DB.QueryContext(r.Context(), `select outbox.activity from notes join outbox on outbox.activity->>'$.object.id' = notes.id where outbox.sender = $1 and outbox.activity->>'$.actor' = $1 and notes.author = $1 and notes.public = 1 and outbox.inserted >= $2 order by notes.inserted limit $3`, actorID.String, since, activitiesPerPage)
	if err != nil {
		l.Log.Warn("Failed to fetch activities", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	defer rows.Close()

	activities := make([]ap.Activity, 0, activitiesPerPage)

	for rows.Next() {
		var activity ap.Activity
		if err := rows.Scan(&activity); err != nil {
			l.Log.Warn("Failed to scan activity", "error", err)
			continue
		}

		activity.Context = nil
		activities = append(activities, activity)
	}
	rows.Close()

	page := map[string]any{
		"@context":     []string{"https://www.w3.org/ns/activitystreams"},
		"id":           fmt.Sprintf("https://%s/outbox/%s?%d", l.Domain, username, since),
		"type":         "OrderedCollectionPage",
		"partOf":       fmt.Sprintf("https://%s/outbox/%s", l.Domain, username),
		"orderedItems": activities,
	}

	var nextSince sql.NullInt64
	if err := l.DB.QueryRowContext(r.Context(), `select max(inserted) from (select outbox.inserted from notes join outbox on outbox.activity->>'$.object.id' = notes.id where outbox.sender = $1 and outbox.activity->>'$.actor' = $1 and notes.author = $1 and notes.public = 1 and outbox.inserted > $2 order by outbox.inserted limit $2 offset $3)`, actorID, since, activitiesPerPage).Scan(&nextSince); err != nil {
		l.Log.Warn("Failed to get next page timestamp", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if nextSince.Valid {
		page["next"] = fmt.Sprintf("https://%s/outbox/%s?%d", l.Domain, username, nextSince.Int64)
	}

	var prevSince sql.NullInt64
	if err := l.DB.QueryRowContext(r.Context(), `select min(inserted) from (select outbox.inserted from notes join outbox on outbox.activity->>'$.object.id' = notes.id where outbox.sender = $1 and outbox.activity->>'$.actor' = $1 and notes.author = $1 and notes.public = 1 and outbox.inserted < $2 order by outbox.inserted desc limit $3)`, actorID, since, activitiesPerPage).Scan(&prevSince); err != nil {
		l.Log.Warn("Failed to get previous page timestamp", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if prevSince.Valid {
		page["prev"] = fmt.Sprintf("https://%s/outbox/%s?%d", l.Domain, username, prevSince.Int64)
	}

	j, err := json.Marshal(page)
	if err != nil {
		l.Log.Warn("Failed to marshal page", "username", username, "since", since, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	w.Write(j)
}
