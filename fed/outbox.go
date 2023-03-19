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
	"strconv"
)

func outboxHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	fmt.Println(r.URL)

	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	name := filepath.Base(r.URL.Path)

	s := r.URL.Query().Get("from")
	if s != "" {
		from, err := strconv.ParseInt(s, 10, 64)
		fmt.Println(err)
		fmt.Println(from)
		if err != nil || from < 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		rows, err := db.Query(`select * from objects where type = 'Note' and Actor = ? limit 10 offset ?;`, fmt.Sprintf("https://%s/user/%s", cfg.Domain, name), from)
		fmt.Println(err)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		defer rows.Close()

		notes := []map[string]any{}

		for rows.Next() {
			o := data.Object{}
			if err = rows.Scan(&o.ID, &o.Type, &o.Actor, &o.Object); err != nil {
				fmt.Println(err)
				continue
			}

			note := map[string]any{}

			if err := json.Unmarshal([]byte(o.Object), &note); err != nil {
				fmt.Println(err)
				continue
			}

			notes = append(notes, map[string]any{
				"id":           o.ID,
				"type":         o.Type,
				"attributedTo": o.Actor,
				"object":       note,
			})
		}

		j, err := json.Marshal(map[string]any{
			"@context":     "https://www.w3.org/ns/activitystreams",
			"id":           fmt.Sprintf("https://%s/outbox/%s?from=%d", cfg.Domain, name, from),
			"type":         "OrderedCollectionPage",
			"prev":         fmt.Sprintf("https://%s/outbox/%s?from=%d", cfg.Domain, name, from-10),
			"partOf":       fmt.Sprintf("https://%s/outbox/%s", cfg.Domain, name),
			"orderedItems": notes,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

		fmt.Println(string(j))
		w.Header().Add("Content-Type", "application/activity+json; charset=utf-8")
		w.Write(j)
		return
	}

	row := db.QueryRow(`select count(*) from objects where type = 'Note' and Actor = ?;`, fmt.Sprintf("https://%s/user/%s", cfg.Domain, name))
	var count int64
	if err := row.Scan(&count); err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	j, err := json.Marshal(map[string]any{
		"@context":   "https://www.w3.org/ns/activitystreams",
		"id":         fmt.Sprintf("https://%s/outbox/%s", cfg.Domain, name),
		"type":       "OrderedCollection",
		"totalItems": count,
		"first":      fmt.Sprintf("https://%s/outbox/%s?from=0", cfg.Domain, name),
		"last":       fmt.Sprintf("https://%s/outbox/%s?from=%d", cfg.Domain, name, count), // TODO: count - 10
	})
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	fmt.Println(string(j))
	w.Header().Add("Content-Type", "application/activity+json; charset=utf-8")
	w.Write(j)
}
