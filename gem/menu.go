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
	"context"
	"database/sql"
	"github.com/dimkr/tootik/data"
	"io"
	"net/url"
)

func writeUserMenu(w io.Writer, user *data.Object) {
	w.Write([]byte("\n────\n\n"))

	if user == nil {
		w.Write([]byte(`=> /public 🏙️ Latest public posts
=> /active 👥 Active users
=> /users 🔑 Sign in`))
		return
	}

	w.Write([]byte(`=> /users 📥 My inbox
=> /users/public 🏙️ Latest public posts
=> /users/active 👥 Active users
=> /users/following 🙆 Followed users
=> /users/post 🖊️ New post
=> /users/public_post ‍🖍️ New public post
`))
}

func withUserMenu(f func(context.Context, io.Writer, *url.URL, []string, *data.Object, *sql.DB)) func(context.Context, io.Writer, *url.URL, []string, *data.Object, *sql.DB) {
	return func(ctx context.Context, w io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
		var buf bytes.Buffer
		f(ctx, &buf, requestUrl, params, user, db)
		writeUserMenu(&buf, user)
		w.Write(buf.Bytes())
	}
}
