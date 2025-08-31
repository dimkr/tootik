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
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text"
)

var gatewayRegex = regexp.MustCompile(`[a-z0-9-]+(?:\.[a-z0-9-]+)+`)

func (h *Handler) portability(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if !ap.IsPortable(r.User.ID) {
		w.Status(40, "Not a portable account")
		return
	}

	w.OK()
	w.Title("🚲 Data Portability")

	w.Subtitle("Private Key")
	w.Text("To register this account on another server, use this Ed25519 private key:")
	w.Empty()
	if r.URL.RawQuery == "show" {
		w.Text(data.EncodeEd25519PrivateKey(r.Keys[1].PrivateKey.(ed25519.PrivateKey)))
	} else {
		w.Text("********")
		w.Link("/users/portability?show", "Show")
	}
	w.Empty()
	w.Text("Then,")
	w.Itemf("Add %s to the list of gateways on the other server", h.Domain)
	w.Item("Add the other server below")
	w.Empty()

	w.Subtitle("Gateways")

	for i, gw := range r.User.Gateways {
		if i > 0 {
			w.Empty()
		}

		gw = strings.TrimPrefix(gw, "https://")
		w.Text(gw)
		if gw != h.Domain {
			w.Link("/users/gateway/remove?"+gw, "➖ Remove")
		}
	}

	w.Empty()
	if len(r.User.Gateways) == h.Config.MaxGateways {
		w.Text("Reached the maximum number of gateways.")
	} else {
		w.Link("/users/gateway/add", "➕ Add")
	}
}

func (h *Handler) gatewayAdd(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if !ap.IsPortable(r.User.ID) {
		w.Status(40, "Not a portable account")
		return
	}

	now := time.Now()

	can := r.User.Published.Time.Add(h.Config.MinActorEditInterval)
	if r.User.Updated != (ap.Time{}) {
		can = r.User.Updated.Time.Add(h.Config.MinActorEditInterval)
	}
	if now.Before(can) {
		r.Log.Warn("Throttled request to add gateway", "can", can)
		w.Statusf(40, "Please wait for %s", time.Until(can).Truncate(time.Second).String())
		return
	}

	if len(r.User.Gateways) >= h.Config.MaxGateways {
		w.Status(40, "Reached the maximum number of gateways")
		return
	}

	if r.URL.RawQuery == "" {
		w.Statusf(10, "Gateway (%s)", h.Domain)
		return
	}

	gw, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Warn("Failed to parse gateway", "raw", r.URL.RawQuery, "error", err)
		w.Status(40, "Bad input")
		return
	}

	if !gatewayRegex.MatchString(gw) {
		r.Log.Warn("Invalid gateway", "gateway", gw)
		w.Status(40, "Bad input")
		return
	}

	if gw == h.Domain {
		w.Status(40, "Cannot add "+h.Domain)
		return
	}

	r.Log.Info("Adding gateway", "gateway", gw)

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to add gateway", "gateway", gw, "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if res, err := tx.ExecContext(
		r.Context,
		`
		update persons
		set actor = jsonb_set(jsonb_insert(actor, '$.gateways[#]', $1), '$.updated', $2)
		where
			id = $3 and
			coalesce(json_array_length(actor->>'$.gateways'), 0) < $4 and
			not exists (select 1 from json_each(actor->'$.gateways') where value = $1)
		`,
		"https://"+gw,
		now.Format(time.RFC3339Nano),
		r.User.ID,
		h.Config.MaxGateways,
	); err != nil {
		r.Log.Error("Failed to add gateway", "gateway", gw, "error", err)
		w.Error()
		return
	} else if one, err := res.RowsAffected(); err != nil {
		r.Log.Error("Failed to add gateway", "gateway", gw, "error", err)
		w.Error()
		return
	} else if one < 1 {
		r.Log.Error("Cannot add gateway", "gateway", gw)
		w.Status(40, "Cannot add gateway")
		return
	}

	if err := h.Inbox.UpdateActor(r.Context, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to add gateway", "gateway", gw, "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to add gateway", "gateway", gw, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/portability")
}

func (h *Handler) gatewayRemove(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if !ap.IsPortable(r.User.ID) {
		w.Status(40, "Not a portable account")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Gateway (example.org)")
		return
	}

	gw, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Warn("Failed to parse gateway", "raw", r.URL.RawQuery, "error", err)
		w.Status(40, "Bad input")
		return
	}

	if gw == h.Domain {
		w.Status(40, "Cannot remove "+h.Domain)
		return
	}

	gw = "https://" + gw

	id := 0
	for i, current := range r.User.Gateways {
		if current == gw {
			id = i
			goto found
		}
	}

	r.Log.Warn("Gateway does not exist", "gw", gw)
	w.Status(40, "Gateway does not exist")
	return

found:
	r.Log.Info("Removing gateway", "gateway", gw)

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to remove gateway", "gateway", gw, "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if res, err := tx.ExecContext(
		r.Context,
		`
		update persons
		set actor = jsonb_set(jsonb_remove(actor, '$.gateways[' || $1 || ']'), '$.updated', $2)
		where
			id = $3 and
			json_extract(actor, '$.gateways[' || $1 || ']') = $4
		`,
		id,
		time.Now().Format(time.RFC3339Nano),
		r.User.ID,
		gw,
	); err != nil {
		r.Log.Error("Failed to remove gateway", "gateway", gw, "id", id, "error", err)
		w.Error()
		return
	} else if one, err := res.RowsAffected(); err != nil {
		r.Log.Error("Failed to remove gateway", "gateway", gw, "id", id, "error", err)
		w.Error()
		return
	} else if one < 1 {
		r.Log.Error("Failed to remove gateway", "gateway", gw, "id", id)
		w.Status(40, "Field does not exist")
		return
	}

	if err := h.Inbox.UpdateActor(r.Context, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to remove gateway", "gateway", gw, "id", id, "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to remove gateway", "gateway", gw, "id", id, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/portability")
}
