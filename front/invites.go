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
	"crypto/ed25519"
	"database/sql"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
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
		SELECT invites.ed25519privkey, invites.inserted, persons.actor, persons.inserted
		FROM invites
		LEFT JOIN persons ON persons.ed25519privkey = invites.ed25519privkey
		WHERE invites.by = $1
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
		var key string
		var inviteInserted int64
		var actor sql.Null[ap.Actor]
		var actorInserted sql.NullInt64
		if err := rows.Scan(&key, &inviteInserted, &actor, &actorInserted); err != nil {
			r.Log.Warn("Failed to scan invite", "error", err)
			continue
		}

		decodedKey, err := data.DecodeEd25519PrivateKey(key)
		if err != nil {
			r.Log.Warn("Failed to decode key", "key", key, "error", err)
			continue
		}

		w.Empty()

		w.Text("ID: " + data.EncodeEd25519PublicKey(decodedKey.Public().(ed25519.PublicKey)))
		w.Text("Created: " + time.Unix(inviteInserted, 0).Format(time.DateOnly))

		if actor.Valid {
			w.Text("Used: " + time.Unix(actorInserted.Int64, 0).Format(time.DateOnly))
			w.Link("/users/outbox/"+strings.TrimPrefix(actor.V.ID, "https://"), "Used by: "+actor.V.PreferredUsername)
		} else {
			w.Link("/users/invite/delete?"+key, "➖ Delete")
		}
	}

	w.Empty()

	if count >= *h.Config.MaxInvitesPerUser {
		w.Text("Reached the maximum number of invitations.")
	} else {
		w.Link("/users/invites/generate", "➕ Invite by newly generated key")
		w.Link("/users/invites/create", "➕ Invite by pre-generated key")
	}
}

func (h *Handler) generateAndInvite(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	_, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		r.Log.Warn("Failed to generate key", "error", err)
		w.Error()
		return
	}

	h.invite(w, r, priv)
}

func (h *Handler) decodeAndInvite(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(11, "base58-encoded Ed25519 private key or 'generate' to generate")
		return
	}

	priv, err := data.DecodeEd25519PrivateKey(r.URL.RawQuery)
	if err != nil {
		r.Log.Warn("Failed to decode key", "error", err)
		w.Error()
		return
	}

	h.invite(w, r, priv)
}

func (h *Handler) invite(w text.Writer, r *Request, priv ed25519.PrivateKey) {
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
		WHERE invites.by = $1 AND NOT EXISTS (SELECT 1 FROM persons WHERE persons.ed25519privkey = invites.ed25519privkey)
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

	if _, err := tx.ExecContext(
		r.Context,
		`
		INSERT INTO invites (ed25519privkey, by)
		VALUES ($1, $2)
		`,
		data.EncodeEd25519PrivateKey(priv),
		r.User.ID,
	); err != nil {
		r.Log.Warn("Failed to insert invite", "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/invites")
}

func (h *Handler) inviteDelete(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(11, "base58-encoded Ed25519 private key")
		return
	}

	if res, err := h.DB.ExecContext(
		r.Context,
		`
		DELETE FROM invites
		WHERE ed25519privkey = $1 AND owner = $2 AND NOT EXISTS (SELECT 1 FROM persons WHERE persons.ed25519privkey = invites.ed25519privkey)
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
		r.Log.Warn("No such invite")
		w.Status(40, "No such invite")
		return
	}

	w.Redirect("/users/invites")
}
