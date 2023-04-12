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
	"bytes"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/text"
)

func writeUserMenu(w text.Writer, user *ap.Actor) {
	w.Separator()

	prefix := ""
	if user != nil {
		prefix = "/users"
	}

	if user != nil {
		w.Link("/users", "📻 My radio")
		w.Link("/users/follows", "⚡️ Followed users")
	}

	w.Link(prefix+"/local", "📡 This planet")
	w.Link(prefix+"/federated", "✨ Outer space")

	if user != nil {
		w.Link("/users/resolve", "🔭 Find user")
	}

	w.Link(prefix+"/active", "🐾 Active users")
	w.Link(prefix+"/instances", "🌕 Other servers")
	w.Link(prefix+"/stats", "📊 Statistics")

	if user == nil {
		w.Link("/users", "🔑 Sign in")
	} else {
		w.Link("/users/whisper", "🔔 New post")
		w.Link("/users/say", "‍📣 New public post")
	}
}

func withUserMenu(f func(text.Writer, *request)) func(text.Writer, *request) {
	return func(w text.Writer, r *request) {
		var buf bytes.Buffer
		prev := w.Wrap(&buf)
		f(w, r)
		writeUserMenu(w, r.User)
		w.Wrap(prev)
		w.Write(buf.Bytes())
	}
}
