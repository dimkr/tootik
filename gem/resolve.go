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

package gem

import (
	"crypto/sha256"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"io"
	"net/url"
	"regexp"
	"strings"
)

func init() {
	handlers[regexp.MustCompile(`^/users/resolve$`)] = withUserMenu(resolve)
}

func resolve(w io.Writer, r *request) {
	if r.URL.RawQuery == "" {
		w.Write([]byte("10 User name (name or name@domain)\r\n"))
		return
	}

	query, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.WithField("url", r.URL.String()).WithError(err).Info("Failed to decode user name")
		w.Write([]byte("40 Bad input\r\n"))
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
		w.Write([]byte("40 Bad input\r\n"))
		return
	}

	actorID := fmt.Sprintf("https://%s/user/%s", host, name)

	r.Log.WithField("id", actorID).Info("Resolving user ID")

	person, err := r.Resolve(actorID)
	if err != nil {
		r.Log.WithField("id", actorID).WithError(err).Warn("Failed to resolve user ID")
		fmt.Fprintf(w, "40 Failed to resolve %s@%s\r\n", name, host)
		return
	}

	fmt.Fprintf(w, "30 /users/outbox/%x\r\n", sha256.Sum256([]byte(person.ID)))
}
