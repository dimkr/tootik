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
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"io/ioutil"
	"log/slog"
	"net/http"
	"path/filepath"
)

const maxBodySize = 1024 * 1024

type inboxHandler struct {
	Log      *slog.Logger
	DB       *sql.DB
	Resolver *Resolver
	Actor    *ap.Actor
}

func (h *inboxHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	receiver := fmt.Sprintf("https://%s/user/%s", cfg.Domain, filepath.Base(r.URL.Path))
	var registered int
	if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where id = ?)`, receiver).Scan(&registered); err != nil {
		h.Log.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if registered == 0 {
		h.Log.Warn("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		return
	}

	var activity ap.Activity
	if err := json.Unmarshal(body, &activity); err != nil {
		h.Log.Warn("Failed to unmarshal activity", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	// if actor is deleted, ignore this activity if we don't know this actor
	offline := false
	if activity.Type == ap.DeleteActivity {
		offline = true
	}

	sender, err := verify(r.Context(), h.Log, r, h.DB, h.Resolver, h.Actor, offline)
	if err != nil {
		if errors.Is(err, ErrActorGone) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrActorNotCached) {
			h.Log.Debug("Ignoring Delete activity for unknown actor", "error", err)
		} else {
			w.WriteHeader(http.StatusUnauthorized)
		}
		return
	}

	if _, err = h.DB.ExecContext(
		r.Context(),
		`INSERT INTO inbox (sender, activity) VALUES(?,?)`,
		sender.ID,
		string(body),
	); err != nil {
		h.Log.Error("Failed to insert activity", "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
