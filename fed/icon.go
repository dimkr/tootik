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
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed/icon"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
)

type iconHandler struct {
	Log *slog.Logger
	DB  *sql.DB
}

func (h *iconHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	name := filepath.Base(r.URL.Path)
	if !strings.HasSuffix(name, icon.FileNameExtension) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	name = name[:len(name)-len(icon.FileNameExtension)]

	actorID := fmt.Sprintf("https://%s/user/%s", cfg.Domain, name)

	h.Log.Info("Looking up cached icon", "id", actorID)

	var cache []byte
	if err := h.DB.QueryRowContext(r.Context(), `select buf from icons where id = ?`, actorID).Scan(&cache); err != nil && !errors.Is(err, sql.ErrNoRows) {
		h.Log.Warn("Failed to get cached icon", actorID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if len(cache) > 0 {
		h.Log.Debug("Sending cached icon", "id", actorID)
		w.Header().Set("Content-Type", icon.MediaType)
		w.Write(cache)
		return
	}

	var exists int
	if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where id = ?)`, actorID).Scan(&exists); err != nil {
		h.Log.Warn("Failed to check if user exists", actorID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		h.Log.Warn("No icon for non-existing user", "id", actorID)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	h.Log.Info("Generating an icon", "id", actorID)

	buf, err := icon.Generate(actorID)
	if err != nil {
		h.Log.Warn("Failed to generate icon", actorID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := h.DB.ExecContext(r.Context(), `insert into icons(id, buf) values(?,?)`, actorID, buf); err != nil {
		h.Log.Warn("Failed to cache icon", "id", actorID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", icon.MediaType)
	w.Write(buf)
}
