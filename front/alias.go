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
	"net/url"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
)

func (h *Handler) alias(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := time.Now()

	can := r.User.Published.Time.Add(h.Config.MinActorEditInterval)
	if r.User.Updated != (ap.Time{}) {
		can = r.User.Updated.Time.Add(h.Config.MinActorEditInterval)
	}
	if now.Before(can) {
		r.Log.Warn("Throttled request to set alias", "can", can)
		w.Statusf(40, "Please wait for %s", time.Until(can).Truncate(time.Second).String())
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Alias (name@domain)")
		return
	}

	alias, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		r.Log.Warn("Failed to decode alias", "query", r.URL.RawQuery, "error", err)
		w.Status(40, "Bad input")
		return
	}

	tokens := strings.SplitN(alias, "@", 3)
	if len(tokens) != 2 {
		r.Log.Warn("Alias is invalid", "alias", alias)
		w.Status(40, "Bad input")
		return
	}

	actor, err := h.Resolver.Resolve(r.Context, r.Keys, tokens[1], tokens[0], 0)
	if err != nil {
		r.Log.Warn("Failed to resolve alias", "alias", alias, "error", err)
		w.Status(40, "Failed to resolve "+alias)
		return
	}

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to update alias", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		r.Context,
		"update persons set actor = jsonb_set(actor, '$.alsoKnownAs', json_array($1), '$.updated', $2) where id = $3",
		actor.ID,
		now.Format(time.RFC3339Nano),
		r.User.ID,
	); err != nil {
		r.Log.Error("Failed to update alias", "error", err)
		w.Error()
		return
	}

	if err := outbox.UpdateActor(r.Context, h.Domain, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to update alias", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to update alias", "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/outbox/" + trimScheme(actor.ID))
}
