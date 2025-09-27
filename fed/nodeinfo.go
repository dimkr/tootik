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
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/lock"
)

const nodeInfoUpdateInterval = time.Hour * 6

func addNodeInfo20Stub(mux *http.ServeMux, closed bool) error {
	body, err := json.Marshal(map[string]any{
		"version": "2.0",
		"software": map[string]any{
			"name":    "tootik",
			"version": buildinfo.Version,
		},
		"protocols": []string{
			"activitypub",
		},
		"services": map[string]any{
			"outbound": []any{},
			"inbound":  []any{},
		},
		"usage": map[string]any{
			"users": map[string]any{
				"total":          0,
				"activeMonth":    0,
				"activeHalfyear": 0,
			},
			"localPosts": 0,
		},
		"openRegistrations": !closed,
		"metadata":          map[string]any{},
	})
	if err != nil {
		return err
	}

	mux.HandleFunc("GET /nodeinfo/2.0", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	})

	return nil
}

func addNodeInfo(mux *http.ServeMux, domain string, closed bool, cfg *cfg.Config, db *sql.DB) error {
	if body, err := json.Marshal(map[string]any{
		"links": []map[string]any{
			{
				"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.0",
				"href": fmt.Sprintf("https://%s/nodeinfo/2.0", domain),
			},
		},
	}); err != nil {
		return err
	} else {
		mux.HandleFunc("GET /.well-known/nodeinfo", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		})
	}

	if !cfg.FillNodeInfoUsage {
		return addNodeInfo20Stub(mux, closed)
	}

	l := lock.New()
	var total, activeHalfYear, activeMonth, localPosts int64
	var last time.Time

	mux.HandleFunc("GET /nodeinfo/2.0", func(w http.ResponseWriter, r *http.Request) {
		if err := l.Lock(r.Context()); err != nil {
			slog.WarnContext(r.Context(), "Failed to build nodeinfo response", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer l.Unlock()

		now := time.Now()
		if now.Sub(last) >= nodeInfoUpdateInterval {
			if err := db.QueryRowContext(
				r.Context(),
				`
				select
					(select count(*)-1 from persons where host = $1),
					(
						select count(*) from
						(
							select distinct author from notes where inserted > unixepoch()-60*60*24*182.5 and host = $1
							union all
							select distinct id from persons where host = $1 and exists (select 1 from shares where shares.inserted > unixepoch()-60*60*24*182.5 and shares.by = persons.id)
						)
					),
					(
						select count(*) from
						(
							select distinct author from notes where inserted > unixepoch()-60*60*24*30 and host = $1
							union all
							select distinct id from persons where host = $1 and exists (select 1 from shares where shares.inserted > unixepoch()-60*60*24*30 and shares.by = persons.id)
						)
					),
					(select count(*) from notes where host = $1)
				`,
				domain,
			).Scan(&total, &activeHalfYear, &activeMonth, &localPosts); err != nil {
				slog.WarnContext(r.Context(), "Failed to build nodeinfo response", "error", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			last = now
		}

		if body, err := json.Marshal(map[string]any{
			"version": "2.0",
			"software": map[string]any{
				"name":    "tootik",
				"version": buildinfo.Version,
			},
			"protocols": []string{
				"activitypub",
			},
			"services": map[string]any{
				"outbound": []any{},
				"inbound":  []any{},
			},
			"usage": map[string]any{
				"users": map[string]any{
					"total":          total,
					"activeMonth":    activeMonth,
					"activeHalfyear": activeHalfYear,
				},
				"localPosts": localPosts,
			},
			"openRegistrations": !closed,
			"metadata":          map[string]any{},
		}); err != nil {
			slog.WarnContext(r.Context(), "Failed to build nodeinfo response", "error", err)
			w.WriteHeader(http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		}
	})

	return nil
}
