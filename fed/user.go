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
	"github.com/dimkr/tootik/cfg"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
)

type userHandler struct {
	Log *slog.Logger
	DB  *sql.DB
}

func (h *userHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	name := filepath.Base(r.URL.Path)
	actorID := fmt.Sprintf("https://%s/user/%s", cfg.Domain, name)

	// redirect browsers to the outbox page over Gemini
	accept := strings.ReplaceAll(r.Header.Get("Accept"), " ", "")
	if strings.HasPrefix(accept, "text/html,") || strings.HasSuffix(accept, ",text/html") || strings.Contains(accept, ",text/html,") {
		outbox := fmt.Sprintf("gemini://%s/outbox/%x", cfg.Domain, sha256.Sum256([]byte(actorID)))
		h.Log.Info("Redirecting to outbox over Gemini", "outbox", outbox)
		w.Header().Set("Location", outbox)
		w.WriteHeader(http.StatusMovedPermanently)
		return
	}

	h.Log.Info("Looking up user", "id", actorID)

	actorString := ""
	if err := h.DB.QueryRowContext(r.Context(), `select actor from persons where id = ?`, actorID).Scan(&actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		h.Log.Info("Notifying about deleted user", "id", actorID)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	actor := map[string]any{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	delete(actor, "privateKey")
	delete(actor, "clientCertificate")

	resp, err := json.Marshal(actor)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write(resp)
}
