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
	"errors"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"path/filepath"
)

type inboxHandler struct {
	Log *log.Logger
	DB  *sql.DB
}

func (h *inboxHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	receiver := fmt.Sprintf("https://%s/user/%s", cfg.Domain, filepath.Base(r.URL.Path))
	var registered int
	if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where id = ?)`, receiver).Scan(&registered); err != nil {
		h.Log.WithField("receiver", receiver).WithError(err).Warn("Failed to check if receiving user exists")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if registered == 0 {
		h.Log.WithField("receiver", receiver).Warn("Receiving user does not exist")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	sender, err := verify(r.Context(), r, h.DB)
	if err != nil {
		if errors.Is(err, goneError) {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.Log.WithError(err).Warn("Failed to verify message")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	if _, err = h.DB.ExecContext(
		r.Context(),
		`INSERT INTO activities (sender, activity) VALUES(?,?)`,
		sender.ID,
		string(body),
	); err != nil {
		h.Log.WithField("sender", sender.ID).WithError(err).Error("Failed to insert activity")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
