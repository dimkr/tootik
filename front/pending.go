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
	"net/url"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) pending(w text.Writer, r *Request, args ...string) {
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
		case "enable":
			if _, err := h.DB.ExecContext(
				r.Context,
				"update persons set actor = json_set(actor, '$.manuallyApprovesFollowers', json('true')) where id = ?",
				r.User.ID,
			); err != nil {
				r.Log.Error("Failed to enable follower approval", "error", err)
				w.Error()
			}

		case "disable":
			if _, err := h.DB.ExecContext(
				r.Context,
				"update persons set actor = json_set(actor, '$.manuallyApprovesFollowers', json('false')) where id = ?",
				r.User.ID,
			); err != nil {
				r.Log.Error("Failed to disable follower approval", "error", err)
				w.Error()
			}

		default:
			w.Status(40, "Bad input")
		}

		return
	}

	w.OK()
	w.Title("⏳ Follow Requests")

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		select inserted, actor from
		(
			select follows.inserted, persons.actor from follows
			join persons on persons.id = follows.follower
			where follows.followed = $1 and follows.accepted is null
			union all
			select follows.inserted, persons.actor from follows
			join persons on persons.id = follows.followed
			where follows.follower = $1 and follows.accepted is null
		)
		order by inserted desc
		`,
		r.User.ID,
	)
	if err != nil {
		r.Log.Warn("Failed to list followers", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	empty := true

	for rows.Next() {
		var follower ap.Actor
		var inserted int64
		if err := rows.Scan(&follower, &inserted); err != nil {
			r.Log.Warn("Failed to list a follow request", "error", err)
			continue
		}

		param := strings.TrimPrefix(follower.ID, "https://")

		w.Linkf(
			"/users/outbox/"+param,
			"%s %s",
			time.Unix(inserted, 0).Format(time.DateOnly),
			h.getActorDisplayName(&follower),
		)

		if follower.ID != r.User.ID {
			w.Link("/users/follows/accept/"+param, "🟢 Accept")
			w.Link("/users/follows/reject/"+param, "🔴 Reject")
		}

		empty = false
	}

	if empty {
		w.Text("No follow requests.")
		w.Empty()
	}

	if r.User.ManuallyApprovesFollowers {
		w.Link("/users/follows/pending?disable", "Disable manual follower approval")
	} else {
		w.Link("/users/follows/pending?enable", "Enable manual follower approval")
	}
}
