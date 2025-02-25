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

import "github.com/dimkr/tootik/front/text"

func (h *Handler) approve(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	hash := args[1]

	r.Log.Info("Approving certificate", "user", r.User.PreferredUsername, "hash", hash)

	if res, err := h.DB.ExecContext(
		r.Context,
		`
		update certificates set approved = 1
		where user = ? and hash = ? and approved = 0
		`,
		r.User.PreferredUsername,
		hash,
	); err != nil {
		r.Log.Warn("Failed to approve certificate", "user", r.User.PreferredUsername, "hash", hash, "error", err)
		w.Error()
		return
	} else if n, err := res.RowsAffected(); err != nil {
		r.Log.Warn("Failed to approve certificate", "user", r.User.PreferredUsername, "hash", hash, "error", err)
		w.Error()
		return
	} else if n == 0 {
		r.Log.Warn("Certificate doesn't exist or already approved", "user", r.User.PreferredUsername, "hash", hash)
		w.Status(40, "Cannot approve certificate")
		return
	}

	w.Redirect("/users/certificates")
}
