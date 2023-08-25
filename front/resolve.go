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

package front

import (
	"crypto/sha256"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/text"
	"net/url"
	"regexp"
	"strings"
)

func init() {
	handlers[regexp.MustCompile(`^/users/resolve$`)] = withUserMenu(resolve)
}

func resolve(w text.Writer, r *request) {
	if r.URL.RawQuery == "" {
		w.Status(10, "User name (name or name@domain)")
		return
	}

	query, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Info("Failed to decode user name", "url", r.URL, "error", err)
		w.Status(40, "Bad input")
		return
	}

	var name, host string

	tokens := strings.Split(query, "@")
	switch len(tokens) {
	case 1:
		name = tokens[0]
		host = cfg.Domain
	case 2:
		name = tokens[0]
		host = tokens[1]
	default:
		w.Status(40, "Bad input")
		return
	}

	actorID := fmt.Sprintf("https://%s/user/%s", host, name)

	r.Log.Info("Resolving user ID", "id", actorID)

	person, err := r.Resolve(actorID, false)
	if err != nil {
		r.Log.Warn("Failed to resolve user ID", "id", actorID, "error", err)
		w.Statusf(40, "Failed to resolve %s@%s", name, host)
		return
	}

	w.Redirectf("/users/outbox/%x", sha256.Sum256([]byte(person.ID)))
}
