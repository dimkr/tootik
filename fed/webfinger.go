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
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
)

func (l *Listener) handleWebFinger(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	if len(query) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("No query"))
		return
	}

	resource, err := url.QueryUnescape(query.Get("resource"))
	if err != nil {
		l.Log.Info("Failed to decode query", "resource", r.URL.RawQuery, "error", err)
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
			l.Log.Info("Received invalid resource", "resource", resource)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("Resource must contain zero or one @"))
			return
		}

		if len(fields) == 2 && fields[1] != l.Domain {
			l.Log.Info("Received invalid resource", "resource", resource, "domain", fields[1])
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Resource must end with @%s", l.Domain)
			return
		}

		username = fields[0]
	}

	l.Log.Info("Looking up resource", "resource", resource, "user", username)

	var actorID sql.NullString
	if err := l.DB.QueryRowContext(r.Context(), `select id from persons where actor->>'preferredUsername' = ? and host = ?`, username, l.Domain).Scan(&actorID); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !actorID.Valid {
		l.Log.Info("Notifying that user does not exist", "user", username)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	j, err := json.Marshal(map[string]any{
		"subject": fmt.Sprintf("acct:%s@%s", username, l.Domain),
		"aliases": []string{actorID.String},
		"links": []map[string]any{
			{
				"rel":  "self",
				"type": "application/activity+json",
				"href": actorID.String,
			},
			{
				"rel":  "self",
				"type": `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`,
				"href": actorID.String,
			},
		},
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/jrd+json; charset=utf-8")
	w.Write(j)
}
