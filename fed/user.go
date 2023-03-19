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
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	_ "github.com/mattn/go-sqlite3"
	"net/http"
	"path/filepath"
)

type webFingerResponse struct {
	Context           []string `json:"@context"`
	ID                string   `json:"id"`
	Type              string   `json:"type"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferredUsername"`
	Inbox             string   `json:"inbox"`
	Outbox            string   `json:"outbox"`
	PublicKey         map[string]any
	Discoverable      bool `json:"discoverable"`
}

func userHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	name := filepath.Base(r.URL.Path)

	u, err := data.Objects.GetByID(fmt.Sprintf("https://%s/user/%s", cfg.Domain, name), db)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	j := map[string]any{}
	if err := json.Unmarshal([]byte(u.Object), &j); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	delete(j, "privateKey")
	delete(j, "clientCertificate")

	resp, err := json.Marshal(j)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	w.Write(resp)
}
