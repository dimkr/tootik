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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"log/slog"
	"net/http"
	"path/filepath"
)

type postHandler struct {
	Log *slog.Logger
	DB  *sql.DB
}

func (h *postHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	postID := fmt.Sprintf("https://%s/post/%s", cfg.Domain, filepath.Base(r.URL.Path))

	if shouldRedirect(r) {
		outbox := fmt.Sprintf("gemini://%s/view/%x", cfg.Domain, sha256.Sum256([]byte(postID)))
		h.Log.Info("Redirecting to outbox over Gemini", "outbox", outbox)
		w.Header().Set("Location", outbox)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	h.Log.Info("Fetching post", "post", postID)

	var noteString string
	if err := h.DB.QueryRowContext(r.Context(), `select object from notes where id = ? and public = 1`, postID).Scan(&noteString); err != nil && errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		h.Log.Warn("Failed to fetch post", "post", postID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var note ap.Object
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		h.Log.Warn("Failed to unmarshal post", "post", postID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	note.Context = "https://www.w3.org/ns/activitystreams"

	j, err := json.Marshal(note)
	if err != nil {
		h.Log.Warn("Failed to marshal post", "post", postID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/activity+json; charset=utf-8")
	w.Write(j)
}
