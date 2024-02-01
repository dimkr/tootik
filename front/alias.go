/*
Copyright 2024 Dima Krasner

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
	"fmt"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
	"net/url"
	"strings"
	"time"
)

func (h *Handler) alias(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := time.Now()

	if (r.User.Updated != nil && now.Sub(r.User.Updated.Time) < h.Config.MinActorEditInterval) || (r.User.Updated == nil && now.Sub(r.User.Published.Time) < h.Config.MinActorEditInterval) {
		r.Log.Warn("Throttled request to set alias")
		w.Status(40, "Please try again later")
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

	actor, err := r.Resolve(fmt.Sprintf("https://%s/user/%s", tokens[1], tokens[0]), false)
	if err != nil {
		r.Log.Warn("Failed to resolve alias", "alias", alias, "error", err)
		w.Status(40, "Failed to resolve "+alias)
		return
	}

	tx, err := r.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to update alias", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		r.Context,
		"update persons set actor = json_set(actor, '$.alsoKnownAs', json_array($1), '$.updated', $2) where id = $3",
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

	w.Redirect("/users/outbox/" + strings.TrimPrefix(r.User.ID, "https://"))
}
