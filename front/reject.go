/*
Copyright 2025, 2026 Dima Krasner

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
	"database/sql"
	"errors"

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) reject(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	arg := args[1]

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to reject follow request", "follower", arg, "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	var follower, followID string
	if err := tx.QueryRowContext(
		r.Context,
		`SELECT follows.follower, follows.id FROM follows JOIN persons ON persons.id = follows.follower WHERE (persons.id = 'https://' || $1 OR persons.slug = $1) AND follows.followed = $2`,
		arg,
		r.User.ID,
	).Scan(&follower, &followID); errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Failed to fetch follow request to reject", "follower", arg)
		w.Status(40, "No such follow request")
		return
	} else if err != nil {
		r.Log.Warn("Failed to reject follow request", "follower", arg, "error", err)
		w.Error()
		return
	}

	if err := h.Inbox.Reject(r.Context, r.User, r.Keys[1], follower, followID, tx); err != nil {
		r.Log.Warn("Failed to reject follow request", "follower", follower, "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Warn("Failed to reject follow request", "follower", follower, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/followers")
}
