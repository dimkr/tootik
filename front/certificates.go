/*
Copyright 2024 - 2026 Dima Krasner

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

	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) certificates(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	rows, err := data.QueryCollectRowsIgnore[struct {
		Inserted, Expires int64
		Hash              string
		Approved          int
	}](
		r.Context,
		h.DB,
		func(err error) bool {
			r.Log.Warn("Failed to fetch certificate", "user", r.User.PreferredUsername, "error", err)
			return true
		},
		`
		select inserted, hash, approved, expires from certificates
		where user = ?
		order by inserted
		`,
		r.User.PreferredUsername,
	)

	if err != nil {
		r.Log.Warn("Failed to fetch certificates", "user", r.User.PreferredUsername, "error", err)
		w.Error()
		return
	}

	w.OK()
	w.Title("🎓 Certificates")

	for i, row := range rows {
		if i > 0 {
			w.Empty()
		}

		w.Item("SHA-256: " + row.Hash)
		w.Item("Added: " + time.Unix(row.Inserted, 0).Format(time.DateOnly))
		w.Item("Expires: " + time.Unix(row.Expires, 0).Format(time.DateOnly))

		if row.Approved == 0 {
			w.Link("/users/certificates/approve/"+row.Hash, "🟢 Approve")
			w.Link("/users/certificates/revoke/"+row.Hash, "🔴 Deny")
		} else {
			w.Link("/users/certificates/revoke/"+row.Hash, "🔴 Revoke")
		}
	}
}
