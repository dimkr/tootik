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
	"log/slog"
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
		select inserted, hash, approved, expires from certificates
		where user = ?
		order by inserted
		`,
		r.User.PreferredUsername,
	)
	if err != nil {
		slog.WarnContext(r.Context, "Failed to fetch certificates", "user", r.User.PreferredUsername, "error", err)
		w.Error()
		return
	}

	defer rows.Close()

	w.OK()
	w.Title("🎓 Certificates")

	first := true
	for rows.Next() {
		var inserted, expires int64
		var hash string
		var approved int
		if err := rows.Scan(&inserted, &hash, &approved, &expires); err != nil {
			slog.WarnContext(r.Context, "Failed to fetch certificate", "user", r.User.PreferredUsername, "error", err)
			continue
		}

		if !first {
			w.Empty()
		}

		w.Item("SHA-256: " + hash)
		w.Item("Added: " + time.Unix(inserted, 0).Format(time.DateOnly))
		w.Item("Expires: " + time.Unix(expires, 0).Format(time.DateOnly))

		if approved == 0 {
			w.Link("/users/certificates/approve/"+hash, "🟢 Approve")
			w.Link("/users/certificates/revoke/"+hash, "🔴 Deny")
		} else {
			w.Link("/users/certificates/revoke/"+hash, "🔴 Revoke")
		}

		first = false
	}
}
