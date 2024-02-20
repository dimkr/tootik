/*
Copyright 2023, 2024 Dima Krasner

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
	"net/http"
)

func addNodeInfo(mux *http.ServeMux, domain string, closed bool) error {
	if body, err := json.Marshal(map[string]any{
		"links": map[string]any{
			"rel":  "http://nodeinfo.diaspora.software/ns/schema/2.0",
			"href": fmt.Sprintf("https://%s/nodeinfo/2.0", domain),
		},
	}); err != nil {
		return err
	} else {
		mux.HandleFunc("GET /.well-known/nodeinfo", func(w http.ResponseWriter, r *http.Request) {
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
		"openRegistrations": !closed,
		"metadata":          map[string]any{},
	}); err != nil {
		return err
	} else {
		mux.HandleFunc("GET /nodeinfo/2.0", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Write(body)
		})
	}

	return nil
}
