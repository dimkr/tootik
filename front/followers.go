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
	"database/sql"
	"net/url"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) followers(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery != "" {
		action, err := url.QueryUnescape(r.URL.RawQuery)
		if err != nil {
			w.Status(40, "Bad input")
			return
		}

		tx, err := h.DB.BeginTx(r.Context, nil)
		if err != nil {
			r.Log.Warn("Failed to toggle manual approval", "error", err)
			w.Error()
			return
		}

		switch action {
		case "lock":
			if _, err := tx.ExecContext(
				r.Context,
				"update persons set actor = jsonb_set(actor, '$.manuallyApprovesFollowers', jsonb('true')) where id = ?",
				ap.Canonical(r.User.ID),
			); err != nil {
				r.Log.Warn("Failed to toggle manual approval", "error", err)
				w.Error()
				return
			}

		case "unlock":
			if _, err := tx.ExecContext(
				r.Context,
				"update persons set actor = jsonb_set(actor, '$.manuallyApprovesFollowers', jsonb('false')) where id = ?",
				ap.Canonical(r.User.ID),
			); err != nil {
				r.Log.Warn("Failed to toggle manual approval", "error", err)
				w.Error()
				return
			}

		default:
			w.Status(40, "Bad input")
			return
		}

		if err := h.Queue.UpdateActor(r.Context, tx, r.User.ID); err != nil {
			r.Log.Warn("Failed to toggle manual approval", "error", err)
			w.Error()
			return
		}

		if err := tx.Commit(); err != nil {
			r.Log.Warn("Failed to toggle manual approval", "error", err)
			w.Error()
			return
		}

		w.Redirect("/users/followers")
		return
	}

	w.OK()
	w.Title("🐕 Followers")

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select follows.inserted, json(persons.actor), follows.accepted from follows
		join persons on persons.id = follows.follower
		where follows.followed = $1 and (accepted is null or accepted = 1)
		order by follows.inserted desc
		`,
		ap.Canonical(r.User.ID),
	)
	if err != nil {
		r.Log.Warn("Failed to list followers", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	empty := true

	for rows.Next() {
		var inserted int64
		var follower ap.Actor
		var accepted sql.NullInt32
		if err := rows.Scan(&inserted, &follower, &accepted); err != nil {
			r.Log.Warn("Failed to list a follow request", "error", err)
			continue
		}

		if !empty {
			w.Empty()
		}

		param := strings.TrimPrefix(follower.ID, "https://")

		w.Linkf(
			"/users/outbox/"+param,
			"%s %s",
			time.Unix(inserted, 0).Format(time.DateOnly),
			h.getActorDisplayName(&follower),
		)

		if !accepted.Valid || accepted.Int32 == 0 {
			w.Link("/users/followers/accept/"+param, "🟢 Accept")
		}
		if !accepted.Valid || accepted.Int32 == 1 {
			w.Link("/users/followers/reject/"+param, "🔴 Reject")
		}

		empty = false
	}

	if empty {
		w.Text("No follow requests.")
	}

	w.Empty()
	w.Subtitle("Settings")

	if r.User.ManuallyApprovesFollowers {
		w.Link("/users/followers?unlock", "🔓 Approve new follow requests automatically")
	} else {
		w.Link("/users/followers?lock", "🔒 Approve new follow requests manually")
	}
}
