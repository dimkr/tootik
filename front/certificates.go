/*
Copyright 2024 Dima Krasner

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
	"time"

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) certificates(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select inserted, hash, approved from certificates
		where user = ?
		order by inserted
		`,
		r.User.PreferredUsername,
	)
	if err != nil {
		r.Log.Warn("Failed to fetch certificates", "user", r.User.PreferredUsername, "error", err)
		w.Error()
	}

	defer rows.Close()

	w.OK()
	w.Title("🎓 Certificates")

	for rows.Next() {
		var inserted int64
		var hash string
		var approved int
		if err := rows.Scan(&inserted, &hash, &approved); err != nil {
			r.Log.Warn("Failed to fetch certificate", "user", r.User.PreferredUsername, "error", err)
			continue
		}

		w.Textf("%s %s", time.Unix(inserted, 0).Format(time.DateOnly), hash)
		if approved == 0 {
			w.Linkf("/users/approve/"+hash, "🟢 Approve %s", hash)
			w.Linkf("/users/revoke/"+hash, "🔴 Deny %s", hash)
		} else {
			w.Linkf("/users/revoke/"+hash, "🔴 Revoke %s", hash)
		}
	}
}
