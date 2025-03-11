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
	"time"

	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) bookmark(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/login")
		return
	}

	postID := "https://" + args[1]

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to insert bookmark", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	var exists int
	if err := tx.QueryRowContext(
		r.Context,
		`select exists (
			select 1 from notes
			where
				notes.id = $1 and
				(
					notes.author = $2 or
					notes.public = 1 or
					exists (select 1 from json_each(notes.object->'$.to') where exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = $2 and follows.followed = notes.author and (notes.author = value or persons.actor->>'$.followers' = value))) or
					exists (select 1 from json_each(notes.object->'$.cc') where exists (select 1 from follows join persons on persons.id = follows.followed where follows.follower = $2 and follows.followed = notes.author and (notes.author = value or persons.actor->>'$.followers' = value))) or
					exists (select 1 from json_each(notes.object->'$.to') where value = $2) or
					exists (select 1 from json_each(notes.object->'$.cc') where value = $2)
				)
				
		)`,
		postID,
		r.User.ID,
	).Scan(&exists); err != nil {
		r.Log.Warn("Failed to check if bookmarked post exists", "post", postID, "error", err)
		w.Error()
		return
	} else if exists == 0 {
		r.Log.Info("Post was not found", "post", postID)
		w.Status(40, "Post not found")
		return
	}

	now := time.Now()

	var count int
	var last sql.NullInt64
	if err := tx.QueryRowContext(r.Context, `select count(*), max(inserted) from bookmarks where by = ?`, r.User.ID).Scan(&count, &last); err != nil {
		r.Log.Warn("Failed to check if bookmark needs to be throttled", "error", err)
		w.Error()
		return
	}

	if count >= h.Config.MaxBookmarksPerUser {
		r.Log.Warn("User has reached bookmarks limit", "post", postID)
		w.Status(40, "Reached bookmarks limit")
		return
	}

	if last.Valid {
		t := time.Unix(last.Int64, 0)
		if now.Sub(t) < h.Config.MinBookmarkInterval {
			r.Log.Warn("User is bookmarking too frequently")
			w.Status(40, "Please wait before bookmarking")
			return
		}
	}

	if _, err := tx.ExecContext(r.Context, `insert into bookmarks(note, by) values(?, ?)`, postID, r.User.ID); err != nil {
		r.Log.Warn("Failed to insert bookmark", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Warn("Failed to insert bookmark", "error", err)
		w.Error()
		return
	}

	w.Redirectf("/login/view/" + args[1])
}
