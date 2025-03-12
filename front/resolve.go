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

package front

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

var resolveInputRegex = regexp.MustCompile(`^(\!{0,1})([^@]+)(?:@([^.@]+\.[^@]+)){0,1}$`)

func (h *Handler) resolve(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "User name (name, name@domain or !group@domain)")
		return
	}

	query, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Info("Failed to decode user name", "url", r.URL, "error", err)
		w.Status(40, "Bad input")
		return
	}

	match := resolveInputRegex.FindStringSubmatch(query)
	if match == nil {
		w.Status(40, "Bad input")
		return
	}

	var flags ap.ResolverFlag
	if match[1] == "!" {
		flags |= ap.GroupActor
	}

	name := match[2]

	host := match[3]
	if host == "" {
		host = h.Domain
	}

	r.Log.Info("Resolving user ID", "host", host, "name", name)

	person, err := h.Resolver.Resolve(r.Context, r.Key, host, name, flags)
	if err != nil {
		r.Log.Warn("Failed to resolve user ID", "host", host, "name", name, "error", err)
		w.Statusf(40, "Failed to resolve %s@%s", name, host)
		return
	}

	w.Redirect("/users/outbox/" + strings.TrimPrefix(person.ID, "https://"))
}
