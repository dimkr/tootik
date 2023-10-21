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
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"net/http"
)

func addNodeInfo(mux *http.ServeMux) error {
	if body, err := json.Marshal(map[string]any{
		"links": map[string]any{
			"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.0",
			"href": fmt.Sprintf("https://%s/nodeinfo/2.0", cfg.Domain),
		},
	}); err != nil {
		return err
	} else {
		mux.HandleFunc("/.well-known/nodeinfo", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		})
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
				"total":          0,
				"activeMonth":    0,
				"activeHalfyear": 0,
			},
			"localPosts": 0,
		},
		"openRegistrations": true,
		"metadata":          map[string]any{},
	}); err != nil {
		return err
	} else {
		mux.HandleFunc("/nodeinfo/2.0", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		})
	}

	if body, err := json.Marshal(map[string]any{
		"uri":               cfg.Domain,
		"title":             "tootik",
		"short_description": "Federated nanoblogging service for the small internet",
		"description":       "",
		"email":             "",
		"version":           buildinfo.Version,
		"stats": map[string]any{
			"user_count":   0,
			"status_count": 0,
			"domain_count": 0,
		},
		"registrations":     true,
		"approval_required": false,
		"configuration": map[string]any{
			"statuses": map[string]any{
				"max_characters":        cfg.MaxPostsLength,
				"max_media_attachments": 0,
			},
		},
		"contact_account": map[string]any{
			"username":     "nobody",
			"acct":         "nobody",
			"display_name": "nobody",
		},
	}); err != nil {
		return err
	} else {
		mux.HandleFunc("/api/v1/instance", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		})
	}

	return nil
}
