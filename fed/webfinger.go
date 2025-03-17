/*
Copyright 2023 - 2025 Dima Krasner

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
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/dimkr/tootik/ap"
)

type webFingerProperties struct {
	Type ap.ActorType `json:"https://www.w3.org/ns/activitystreams#type"`
}

type webFingerLink struct {
	Rel        string              `json:"rel"`
	Type       string              `json:"type"`
	Href       string              `json:"href"`
	Properties webFingerProperties `json:"properties"`
}

type webFingerResponse struct {
	Subject string          `json:"subject"`
	Links   []webFingerLink `json:"links"`
}

func (l *Listener) handleWebFinger(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if len(query) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No query"))
		return
	}

	resource, err := url.QueryUnescape(query.Get("resource"))
	if err != nil {
		slog.Info("Failed to decode query", "resource", r.URL.RawQuery, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resource = strings.TrimPrefix(resource, "acct:")

	var username string

	prefix := fmt.Sprintf("https://%s/", l.Domain)
	if strings.HasPrefix(resource, prefix) {
		username = filepath.Base(resource)
	} else {
		var fields = strings.Split(resource, "@")

		if len(fields) > 2 {
			slog.Info("Received invalid resource", "resource", resource)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Resource must contain zero or one @"))
			return
		}

		if len(fields) == 2 && fields[1] != l.Domain {
			slog.Info("Received invalid resource", "resource", resource, "domain", fields[1])
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Resource must end with @%s", l.Domain)
			return
		}

		username = fields[0]
	}

	// nobody is our equivalent of the Mastodon "instance actor"
	if username == l.Domain {
		username = "nobody"
	}

	slog.Info("Looking up resource", "resource", resource, "user", username)

	rows, err := l.DB.QueryContext(r.Context(), `select id, actor->>'$.type' from persons where actor->>'$.preferredUsername' = ? and host = ?`, username, l.Domain)
	if err != nil {
		slog.Warn("Failed to fetch user", "user", username, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	resp := webFingerResponse{
		Subject: fmt.Sprintf("acct:%s@%s", username, l.Domain),
	}

	for rows.Next() {
		var actorID sql.NullString
		var actorType string
		if err := rows.Scan(&actorID, &actorType); err != nil {
			slog.Warn("Failed to scan user", "user", username, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp.Links = append(resp.Links, webFingerLink{
			Rel:  "self",
			Type: "application/activity+json",
			Href: actorID.String,
			Properties: webFingerProperties{
				Type: ap.ActorType(actorType),
			},
		})
	}

	if len(resp.Links) == 0 {
		slog.Info("Notifying that user does not exist", "user", username)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	j, err := json.Marshal(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/jrd+json; charset=utf-8")
	w.Write(j)
}
