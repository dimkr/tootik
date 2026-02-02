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
	"net/url"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/dbx"
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

		switch action {
		case "lock":
			r.User.ManuallyApprovesFollowers = true

		case "unlock":
			r.User.ManuallyApprovesFollowers = false

		default:
			w.Status(40, "Bad input")
			return
		}

		if err := h.Inbox.UpdateActor(r.Context, r.User, r.Keys[1]); err != nil {
			r.Log.Warn("Failed to toggle manual approval", "error", err)
			w.Error()
			return
		}

		w.Redirect("/users/followers")
		return
	}

	rows, err := dbx.QueryCollectRowsIgnore[struct {
		Inserted int64
		Follower ap.Actor
		Accepted sql.NullInt32
	}](
		r.Context,
		h.DB,
		func(err error) bool {
			r.Log.Warn("Failed to list a follow request", "error", err)
			return true
		},
		`
		select follows.inserted, json(persons.actor), follows.accepted from follows
		join persons on persons.id = follows.follower
		where follows.followed = $1 and (accepted is null or accepted = 1)
		order by follows.inserted desc
		`,
		r.User.ID,
	)
	if err != nil {
		r.Log.Warn("Failed to list followers", "error", err)
		w.Error()
		return
	}

	w.OK()
	w.Title("🐕 Followers")

	if len(rows) == 0 {
		w.Text("No follow requests.")
	} else {
		for i, row := range rows {
			if i > 0 {
				w.Empty()
			}

			param := strings.TrimPrefix(row.Follower.ID, "https://")

			w.Linkf(
				"/users/outbox/"+param,
				"%s %s",
				time.Unix(row.Inserted, 0).Format(time.DateOnly),
				h.getActorDisplayName(&row.Follower),
			)

			if !row.Accepted.Valid || row.Accepted.Int32 == 0 {
				w.Link("/users/followers/accept/"+param, "🟢 Accept")
			}
			if !row.Accepted.Valid || row.Accepted.Int32 == 1 {
				w.Link("/users/followers/reject/"+param, "🔴 Reject")
			}
		}
	}

	w.Empty()
	w.Subtitle("Settings")

	if r.User.ManuallyApprovesFollowers {
		w.Link("/users/followers?unlock", "🔓 Approve new follow requests automatically")
	} else {
		w.Link("/users/followers?lock", "🔒 Approve new follow requests manually")
	}
}
