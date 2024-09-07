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
		w.Link("/users", "📻 My feed")
		w.Link("/users/mentions", "📞 Mentions")
		w.Link("/users/follows", "⚡️ Followed users")
		w.Link("/users/me", "😈 My profile")
	}

	w.Link(prefix+"/local", "📡 Local feed")

	if user == nil {
		w.Link("/communities", "🏕️ Communities")
		w.Link("/hashtags", "🔥 Hashtags")
		w.Link("/fts", "🔎 Search posts")
	} else {
		w.Link("/users/communities", "🏕️ Communities")
		w.Link("/users/hashtags", "🔥 Hashtags")
		w.Link("/users/resolve", "🔭 View profile")
		w.Link("/users/fts", "🔎 Search posts")
	}

	if user == nil {
		w.Link("/users", "🔑 Sign in")
	} else {
		w.Link("/users/post", "📣 New post")
		w.Link("/users/settings", "⚙️ Settings")
	}

	w.Link(prefix+"/status", "📊 Status")
	w.Link(prefix+"/help", "🛟 Help")
}

func withUserMenu(f func(text.Writer, *Request, ...string)) func(text.Writer, *Request, ...string) {
	return func(w text.Writer, r *Request, args ...string) {
		f(w, r, args...)
		writeUserMenu(w, r.User)
	}
}
