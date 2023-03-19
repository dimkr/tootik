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
	"strings"
)

func webFingerHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	resource := r.URL.Query().Get("resource")

	if !strings.HasPrefix(resource, "acct:") {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Resource must begin with acct:"))
		return
	}

	var fields = strings.Split(resource[5:], "@")

	if len(fields) > 2 {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("Resource must contain zero or one @"))
		return
	}

	if len(fields) == 2 && fields[1] != cfg.Domain {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Resource must end with with @%s", cfg.Domain)
		return
	}

	_, err := data.Objects.GetByID(fmt.Sprintf("https://%s/user/%s", cfg.Domain, fields[0]), db)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	j, err := json.Marshal(map[string]any{
		"subject": fmt.Sprintf("acct:%s@%s", fields[0], cfg.Domain),
		"aliases": []string{fmt.Sprintf("https://%s/user/%s", cfg.Domain, fields[0])},
		"links": []map[string]any{
			{
				"rel":  "self",
				"type": "application/activity+json",
				"href": fmt.Sprintf("https://%s/user/%s", cfg.Domain, fields[0]),
			},
		},
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Add("Content-Type", "application/json")
	w.Write(j)
}
