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

func (h *Handler) delete(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	postID := ap.Canonicalize(ap.Gateway(h.Domain, args[1]))

	var note ap.Object
	if err := h.DB.QueryRowContext(r.Context, `select json(object) from notes where id = ? and author = ?`, postID, r.User.ID).Scan(&note); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Attempted to delete a non-existing post", "post", postID, "error", err)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch post to delete", "post", postID, "error", err)
		w.Error()
		return
	}

	if err := outbox.Delete(r.Context, h.Domain, h.Config, h.DB, &note); err != nil {
		r.Log.Error("Failed to delete post", "note", note.ID, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/outbox/" + trimScheme(r.User.ID))
}
