/*
Copyright 2023 - 2025 Dima Krasner

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

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
)

func (h *Handler) unfollow(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	followed := ap.Abs(args[1])

	var followID string
	if err := h.DB.QueryRowContext(r.Context, `select follows.id from persons join follows on persons.id = follows.followed where persons.id = ? and follows.follower = ?`, followed, r.User.ID).Scan(&followID); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Cannot undo a non-existing follow", "followed", followed, "error", err)
		w.Status(40, "No such follow")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find followed user", "followed", followed, "error", err)
		w.Error()
		return
	}

	if err := outbox.Unfollow(r.Context, h.DB, r.User.ID, followed, followID); err != nil {
		r.Log.Warn("Failed undo follow", "followed", followed, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/outbox/" + args[1])
}
