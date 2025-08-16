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
	"database/sql"
	"errors"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
)

func (h *Handler) unshare(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	postID := ap.Abs(args[1])

	var share ap.Activity
	if err := h.DB.QueryRowContext(r.Context, `select json(activity) from outbox where activity->>'$.actor' = $1 and sender = $1 and activity->>'$.type' = 'Announce' and activity->>'$.object' = $2`, r.User.ID, postID).Scan(&share); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Attempted to unshare non-existing share", "post", postID, "error", err)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch share to unshare", "post", postID, "error", err)
		w.Error()
		return
	}

	if err := outbox.Undo(r.Context, h.DB, &share); err != nil {
		r.Log.Warn("Failed to unshare post", "post", postID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/view/" + args[1])
}
