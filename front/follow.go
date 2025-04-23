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
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
)

func (h *Handler) follow(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	followed := "https://" + args[1]

	var exists int
	if err := h.DB.QueryRowContext(r.Context, `select exists (select 1 from persons where id = ?)`, followed).Scan(&exists); err != nil {
		r.Log.Warn("Failed to check if user exists", "followed", followed, "error", err)
		w.Error()
		return
	}

	if exists == 0 {
		r.Log.Warn("Cannot follow a non-existing user", "followed", followed)
		w.Status(40, "No such user")
		return
	}

	var follows int
	if err := h.DB.QueryRowContext(r.Context, `select count(*) from follows where follower = ?`, r.User.ID).Scan(&follows); err != nil {
		r.Log.Warn("Failed to count follows", "error", err)
		w.Error()
		return
	}

	if follows >= h.Config.MaxFollowsPerUser {
		w.Status(40, "Following too many users")
		return
	}

	var accepted sql.NullInt32
	if err := h.DB.QueryRowContext(r.Context, `select accepted from follows where follower = ? and followed = ?`, r.User.ID, followed).Scan(&accepted); err != nil && !errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Failed to check if user is already followed", "followed", followed, "error", err)
		w.Error()
		return
	}
	if accepted.Valid && accepted.Int32 == 1 {
		w.Statusf(40, "Already following %s", followed)
		return
	} else if !accepted.Valid {
		w.Statusf(40, "Waiting for approval from %s", followed)
		return
	}

	if err := outbox.Follow(r.Context, h.Domain, r.User, followed, h.DB); err != nil {
		r.Log.Warn("Failed to follow user", "followed", followed, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/outbox/" + args[1])
}
