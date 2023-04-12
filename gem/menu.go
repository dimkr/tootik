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
	"bytes"
	"github.com/dimkr/tootik/ap"
	"io"
)

func writeUserMenu(w io.Writer, user *ap.Actor) {
	w.Write([]byte("\n────\n\n"))

	if user == nil {
		w.Write([]byte(`=> /local 📡 This planet
=> /federated ✨ Outer space
=> /active 🐾 Active users
=> /instances 🌕 Other servers
=> /stats ‍📊 Statistics
=> /users 🔑 Sign in`))
		return
	}

	w.Write([]byte(`=> /users 📻 My radio
=> /users/follows ⚡️ Followed users
=> /users/local 📡 This planet
=> /users/federated ✨ Outer space
=> /users/resolve 🔭 Find user
=> /users/active 🐾 Active users
=> /users/instances 🌕 Other servers
=> /users/stats ‍📊 Statistics
=> /users/whisper 🔔 New post
=> /users/say ‍📣 New public post`))
}

func withUserMenu(f func(io.Writer, *request)) func(io.Writer, *request) {
	return func(w io.Writer, r *request) {
		var buf bytes.Buffer
		f(&buf, r)
		writeUserMenu(&buf, r.User)
		w.Write(buf.Bytes())
	}
}
