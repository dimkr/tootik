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
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/google/uuid"
)

func (h *Handler) invites(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	w.OK()
	w.Title("🎟️ Invitations")

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		SELECT invites.id, invites.inserted, JSON(persons.actor), persons.inserted
		FROM invites
		LEFT JOIN persons ON persons.id = invites.invited
		WHERE invites.inviter = $1
		ORDER BY invites.inserted DESC, persons.actor->>'$.id' DESC
		`,
		r.User.ID,
	)
	if err != nil {
		r.Log.Warn("Failed to fetch invites", "error", err)
		w.Error()
		return
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var id string
		var inviteInserted int64
		var actor sql.Null[ap.Actor]
		var actorInserted sql.NullInt64
		if err := rows.Scan(&id, &inviteInserted, &actor, &actorInserted); err != nil {
			r.Log.Warn("Failed to scan invite", "error", err)
			continue
		}

		if count > 0 {
			w.Empty()
		}

		w.Text("ID: " + id)
		w.Text("Created: " + time.Unix(inviteInserted, 0).Format(time.DateOnly))

		if actor.Valid {
			w.Text("Used: " + time.Unix(actorInserted.Int64, 0).Format(time.DateOnly))
			w.Link("/users/outbox/"+strings.TrimPrefix(actor.V.ID, "https://"), "Used by: "+actor.V.PreferredUsername)
		} else {
			w.Link("/users/invite/delete?"+id, "➖ Delete")
		}

		count++
	}

	if count > 0 {
		w.Empty()
	}

	if count >= *h.Config.MaxInvitesPerUser {
		w.Text("Reached the maximum number of invitations.")
	} else {
		w.Link("/users/invites/create", "➕ Create")
	}
}

func (h *Handler) createInvite(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Cannot generate invite", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	var count int
	if err := h.DB.QueryRowContext(
		r.Context,
		`
		SELECT COUNT(*)
		FROM invites
		WHERE invites.inviter = $1 AND NOT EXISTS (SELECT 1 FROM persons WHERE persons.id = invites.invited)
		`,
		r.User.ID,
	).Scan(&count); err != nil {
		r.Log.Warn("Failed to count invites", "error", err)
		w.Error()
		return
	}

	if count >= *h.Config.MaxInvitesPerUser {
		r.Log.Warn("Reached the maximum number of invitations")
		w.Status(40, "Reached the maximum number of invitations")
		return
	}

	u, err := uuid.NewRandom()
	if err != nil {
		r.Log.Warn("Failed to generate invite ID", "error", err)
		w.Error()
		return
	}
	id := u.String()

	if _, err := tx.ExecContext(
		r.Context,
		`
		INSERT INTO invites (id, inviter)
		VALUES ($1, $2)
		`,
		id,
		r.User.ID,
	); err != nil {
		r.Log.Warn("Failed to insert invite", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Warn("Failed to insert invite", "error", err)
		w.Error()
		return
	}

	r.Log.Info("Generated invite", "id", id)

	w.Redirect("/users/invites")
}

func (h *Handler) deleteInvite(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "ID")
		return
	}

	if res, err := h.DB.ExecContext(
		r.Context,
		`
		DELETE FROM invites
		WHERE id = $1 AND inviter = $2 AND NOT EXISTS (SELECT 1 FROM persons WHERE persons.id = invites.invited)
		`,
		r.URL.RawQuery,
		r.User.ID,
	); err != nil {
		r.Log.Warn("Failed to delete invite", "error", err)
		w.Error()
		return
	} else if n, err := res.RowsAffected(); err != nil {
		r.Log.Warn("Failed to delete invite", "error", err)
		w.Error()
		return
	} else if n == 0 {
		r.Log.Warn("No such invite", "id", r.URL.RawQuery)
		w.Status(40, "No such invite")
		return
	}

	w.Redirect("/users/invites")
}

func (h *Handler) acceptInvite(w text.Writer, r *Request, args ...string) {
	if r.CertHash == "" {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Code")
		return
	}

	if res, err := h.DB.ExecContext(
		r.Context,
		`
		UPDATE invites
		SET certhash = $1
		WHERE id = $2 AND certhash IS NULL
		`,
		r.CertHash,
		r.URL.RawQuery,
	); err != nil {
		r.Log.Warn("Failed to accept invite", "error", err)
		w.Error()
		return
	} else if n, err := res.RowsAffected(); err != nil {
		r.Log.Warn("Failed to accept invite", "error", err)
		w.Error()
		return
	} else if n == 0 {
		r.Log.Warn("No such invite", "id", r.URL.RawQuery)
		w.Status(40, "No such invite")
		return
	}

	r.Log.Info("Accepted invite", "id", r.URL.RawQuery, "hash", r.CertHash)

	w.Redirect("/users/register")
}
