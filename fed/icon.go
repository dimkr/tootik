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
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dimkr/tootik/icon"
)

func (l *Listener) handleIcon(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")
	if !strings.HasSuffix(name, icon.FileNameExtension) {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	name = name[:len(name)-len(icon.FileNameExtension)]

	slog.InfoContext(r.Context(), "Looking up cached icon", "name", name)

	var cache []byte
	if err := l.DB.QueryRowContext(r.Context(), `select buf from icons where name = ?`, name).Scan(&cache); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.WarnContext(r.Context(), "Failed to get cached icon", "name", name, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if len(cache) > 0 {
		slog.DebugContext(r.Context(), "Sending cached icon", "name", name)
		w.Header().Set("Content-Type", icon.MediaType)
		w.Write(cache)
		return
	}

	var exists int
	if err := l.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where actor->>'$.preferredUsername' = ? and host = ?)`, name, l.Domain).Scan(&exists); err != nil {
		slog.WarnContext(r.Context(), "Failed to check if user exists", "name", name, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if exists == 0 {
		slog.WarnContext(r.Context(), "No icon for non-existing user", "name", name)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	slog.InfoContext(r.Context(), "Generating an icon", "name", name)

	buf, err := icon.Generate(name)
	if err != nil {
		slog.WarnContext(r.Context(), "Failed to generate icon", "name", name, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := l.DB.ExecContext(r.Context(), `insert into icons(name, buf) values(?,?)`, name, buf); err != nil {
		slog.WarnContext(r.Context(), "Failed to cache icon", "name", name, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", icon.MediaType)
	w.Write(buf)
}
