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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/dimkr/tootik/ap"
)

func (l *Listener) handleActivity(w http.ResponseWriter, r *http.Request, prefix string) {
	activityID := fmt.Sprintf("https://%s/%s/%s", l.Domain, prefix, r.PathValue("hash"))

	slog.Info("Fetching activity", "activity", activityID)

	var raw string
	var activity ap.Activity
	if err := l.DB.QueryRowContext(r.Context(), `select activity, activity as raw from outbox where activity->>'$.id' = ?`, activityID).Scan(&raw, &activity); errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch activity", "activity", activityID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !activity.IsPublic() {
		slog.Warn("Refused attempt to fetch a non-public activity", "activity", activityID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")

	if activity.Type == ap.Update {
		json.NewEncoder(w).Encode(activity.Object)
	} else {
		w.Write([]byte(raw))
	}
}

func (l *Listener) handleCreate(w http.ResponseWriter, r *http.Request) {
	l.handleActivity(w, r, "create")
}

func (l *Listener) handleUpdate(w http.ResponseWriter, r *http.Request) {
	l.handleActivity(w, r, "update")
}
