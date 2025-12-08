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
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dimkr/tootik/icon"
)

func (l *Listener) handleIcon(w http.ResponseWriter, r *http.Request) {
	if name, ok := strings.CutSuffix(r.PathValue("username"), icon.FileNameExtension); ok {
		l.doHandleIcon(w, r, fmt.Sprintf("https://%s/user/%s", l.Domain, name))
	} else {
		w.WriteHeader(http.StatusNotFound)
	}
}

func (l *Listener) doHandleIcon(w http.ResponseWriter, r *http.Request, cid string) {
	slog.Info("Looking up cached icon", "cid", cid)

	var cache []byte
	if err := l.DB.QueryRowContext(r.Context(), `select buf from icons where cid = ?`, cid).Scan(&cache); err != nil && !errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Failed to get cached icon", "cid", cid, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if err == nil && len(cache) > 0 {
		slog.Debug("Sending cached icon", "cid", cid)
		w.Header().Set("Content-Type", icon.MediaType)
		w.Write(cache)
		return
	}

	slog.Info("Generating an icon", "cid", cid)

	buf, err := icon.Generate(cid)
	if err != nil {
		slog.Warn("Failed to generate icon", "cid", cid, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := l.DB.ExecContext(r.Context(), `insert into icons(cid, buf) values($1, $2) on conflict(cid) do update set buf = $2`, cid, buf); err != nil {
		slog.Warn("Failed to cache icon", "cid", cid, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", icon.MediaType)
	w.Write(buf)
}
