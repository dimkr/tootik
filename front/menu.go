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

package front

import (
	"bytes"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

func writeUserMenu(w text.Writer, user *ap.Actor) {
	w.Separator()

	prefix := ""
	if user != nil {
		prefix = "/users"
	}

	if user != nil {
		w.Link("/users", "📻 My radio")
		w.Link("/users/mentions", "📞 Mentions")
		w.Link("/users/follows", "⚡️ Followed users")
		w.Link("/users/me", "😈 My profile")
	}

	w.Link(prefix+"/local", "📡 This planet")
	w.Link(prefix+"/federated", "✨ FOMO from outer space")

	if user == nil {
		w.Link("/hashtags", "🔥 Hashtags")
		w.Link("/fts", "🔎 Search posts")
	} else {
		w.Link("/users/hashtags", "🔥 Hashtags")
		w.Link("/users/resolve", "🔭 Find user")
		w.Link("/users/fts", "🔎 Search posts")
	}

	if user == nil {
		w.Link("/users", "🔑 Sign in")
	} else {
		w.Link("/users/post", "📣 New post")
		w.Link("/users/settings", "⚙️ Settings")
	}

	w.Link(prefix+"/stats", "📊 Statistics")
	w.Link(prefix+"/help", "🛟 Help")
}

func withUserMenu(f func(text.Writer, *request, ...string)) func(text.Writer, *request, ...string) {
	return func(w text.Writer, r *request, args ...string) {
		var buf bytes.Buffer
		clone := w.Clone(&buf)
		f(clone, r, args...)
		writeUserMenu(clone, r.User)
		w.Write(buf.Bytes())
	}
}
