/*
Copyright 2025 Dima Krasner

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
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
)

func (h *Handler) reject(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	follower := "https://" + args[1]

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to reject follow request", "follower", follower, "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if err := outbox.Reject(r.Context, h.Domain, r.User.ID, follower, h.DB); err != nil {
		r.Log.Warn("Failed to reject follow request", "follower", follower, "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Warn("Failed to reject follow request", "follower", follower, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/follows/pending")
}
