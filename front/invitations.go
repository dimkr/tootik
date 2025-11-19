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

func (h *Handler) invitations(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	w.OK()
	w.Title("🎟️ Invitations")

	rows, err := h.DB.QueryContext(
		r.Context,
		`
		SELECT invites.code, invites.inserted, JSON(persons.actor), persons.inserted
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
	unused := 0
	for rows.Next() {
		var code string
		var inviteInserted int64
		var actor sql.Null[ap.Actor]
		var actorInserted sql.NullInt64
		if err := rows.Scan(&code, &inviteInserted, &actor, &actorInserted); err != nil {
			r.Log.Warn("Failed to scan invitation", "error", err)
			continue
		}

		if count > 0 {
			w.Empty()
		}

		w.Text("Code: " + code)
		w.Text("Created: " + time.Unix(inviteInserted, 0).Format(time.DateOnly))

		if actor.Valid {
			w.Text("Used: " + time.Unix(actorInserted.Int64, 0).Format(time.DateOnly))
			w.Link("/users/outbox/"+strings.TrimPrefix(actor.V.ID, "https://"), "Used by: "+actor.V.PreferredUsername)
		} else {
			w.Link("/users/invitations/delete?"+code, "➖ Delete")
			unused++
		}

		count++
	}

	if count > 0 {
		w.Empty()
	}

	if unused >= *h.Config.MaxInvitationsPerUser {
		w.Text("Reached the maximum number of invitations.")
	} else {
		w.Link("/users/invitations/create", "➕ Create")
	}
}

func (h *Handler) createInvitation(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	code := r.URL.RawQuery
	if code == "" {
		if u, err := uuid.NewRandom(); err != nil {
			r.Log.Warn("Failed to generate invitation code", "error", err)
			w.Error()
			return
		} else {
			code = u.String()
		}
	} else if err := uuid.Validate(r.URL.RawQuery); err != nil {
		r.Log.Warn("Invitation code is invalid", "code", r.URL.RawQuery, "error", err)
		w.Status(40, "Invalid invitation code")
		return
	}

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Cannot generate invitation", "error", err)
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
		WHERE inviter = $1 AND certhash IS NULL
		`,
		r.User.ID,
	).Scan(&count); err != nil {
		r.Log.Warn("Failed to count invites", "error", err)
		w.Error()
		return
	}

	if count >= *h.Config.MaxInvitationsPerUser {
		r.Log.Warn("Reached the maximum number of invitations")
		w.Status(40, "Reached the maximum number of invitations")
		return
	}

	if _, err := tx.ExecContext(
		r.Context,
		`
		INSERT INTO invites (code, inviter)
		VALUES ($1, $2)
		`,
		code,
		r.User.ID,
	); err != nil {
		r.Log.Warn("Failed to insert invitation", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Warn("Failed to insert invitation", "error", err)
		w.Error()
		return
	}

	r.Log.Info("Generated invitation", "code", code)

	w.Redirect("/users/invitations")
}

func (h *Handler) deleteInvitation(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
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
		DELETE FROM invites
		WHERE code = $1 AND inviter = $2 AND NOT EXISTS (SELECT 1 FROM persons WHERE persons.id = invites.invited)
		`,
		r.URL.RawQuery,
		r.User.ID,
	); err != nil {
		r.Log.Warn("Failed to delete invitation", "error", err)
		w.Error()
		return
	} else if n, err := res.RowsAffected(); err != nil {
		r.Log.Warn("Failed to delete invitation", "error", err)
		w.Error()
		return
	} else if n == 0 {
		r.Log.Warn("Invalid invitation code", "code", r.URL.RawQuery)
		w.Status(40, "Invalid invitation code")
		return
	}

	w.Redirect("/users/invitations")
}

func (h *Handler) acceptInvitation(w text.Writer, r *Request, args ...string) {
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
		WHERE code = $2 AND certhash IS NULL
		`,
		r.CertHash,
		r.URL.RawQuery,
	); err != nil {
		r.Log.Warn("Failed to accept invitation", "error", err)
		w.Error()
		return
	} else if n, err := res.RowsAffected(); err != nil {
		r.Log.Warn("Failed to accept invitation", "error", err)
		w.Error()
		return
	} else if n == 0 {
		r.Log.Warn("Invalid invitation code", "code", r.URL.RawQuery)
		w.Status(40, "Invalid invitation code")
		return
	}

	r.Log.Info("Accepted invitation", "code", r.URL.RawQuery, "hash", r.CertHash)

	w.Redirect("/users/register")
}
