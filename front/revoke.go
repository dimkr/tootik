/*
Copyright 2024, 2025 Dima Krasner

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

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) revoke(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	hash := args[1]

	slog.InfoContext(r.Context, "Revoking certificate", "user", r.User.PreferredUsername, "hash", hash)

	if res, err := h.DB.ExecContext(
		r.Context,
		`
		delete from certificates
		where user = $1 and hash = $2 and exists (select 1 from certificates others where others.user = $1 and others.hash != $2 and others.approved = 1)
		`,
		r.User.PreferredUsername,
		hash,
	); err != nil {
		slog.WarnContext(r.Context, "Failed to revoke certificate", "user", r.User.PreferredUsername, "hash", hash, "error", err)
		w.Error()
		return
	} else if n, err := res.RowsAffected(); err != nil {
		slog.WarnContext(r.Context, "Failed to revoke certificate", "user", r.User.PreferredUsername, "hash", hash, "error", err)
		w.Error()
		return
	} else if n == 0 {
		slog.WarnContext(r.Context, "Certificate doesn't exist or already revoked", "user", r.User.PreferredUsername, "hash", hash)
		w.Status(40, "Cannot revoke certificate")
		return
	}

	w.Redirect("/users/certificates")
}
