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
	"log/slog"
	"net/url"

	"github.com/dimkr/tootik/front/text"
)

func search(w text.Writer, r *Request, args ...string) {
	if r.URL.RawQuery == "" {
		w.Status(10, "Hashtag")
		return
	}

	hashtag, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		slog.InfoContext(r.Context, "Failed to decode query", "url", r.URL, "error", err)
		w.Status(40, "Bad input")
		return
	}

	if r.User == nil && hashtag[0] == '#' {
		w.Redirect("/hashtag/" + hashtag[1:])
	} else if r.User == nil {
		w.Redirect("/hashtag/" + hashtag)
	} else if hashtag[0] == '#' {
		w.Redirect("/users/hashtag/" + hashtag[1:])
	} else {
		w.Redirect("/users/hashtag/" + hashtag)
	}
}
