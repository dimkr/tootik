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
	"github.com/dimkr/tootik/icon"
	log "github.com/dimkr/tootik/slogru"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"path/filepath"
	"strings"
)

type iconHandler struct {
	Log *log.Logger
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

	h.Log.WithField("id", actorID).Info("Looking up cached icon")

	var cache []byte
	if err := h.DB.QueryRowContext(r.Context(), `select buf from icons where id = ?`, actorID).Scan(&cache); err != nil && !errors.Is(err, sql.ErrNoRows) {
		h.Log.WithField("id", actorID).WithError(err).Warn("Failed to get cached icon")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if len(cache) > 0 {
		h.Log.WithField("id", actorID).Debug("Sending cached icon")
		w.Header().Set("Content-Type", icon.MediaType)
		w.Write(cache)
		return
	}

	var exists int
	if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where id = ?)`, actorID).Scan(&exists); err != nil {
		h.Log.WithField("id", actorID).WithError(err).Warn("Failed to check if user exists")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		h.Log.WithField("id", actorID).Warn("No icon for non-existing user")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	h.Log.WithField("id", actorID).Info("Generating an icon")

	buf, err := icon.Generate(actorID)
	if err != nil {
		h.Log.WithField("id", actorID).WithError(err).Warn("Failed to generate icon")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := h.DB.ExecContext(r.Context(), `insert into icons(id, buf) values(?,?)`, actorID, buf); err != nil {
		h.Log.WithField("id", actorID).WithError(err).Warn("Failed to cache icon")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", icon.MediaType)
	w.Write(buf)
}
