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
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
	"time"
)

func (h *Handler) shouldThrottleBoost(r *request) (bool, error) {
	now := time.Now()

	var today, last sql.NullInt64
	if err := r.QueryRow(`select count(*), max(inserted) from outbox where activity->>'actor' = ? and (activity->>'type' = 'Announce' or activity->>'type' = 'Undo') and inserted > ?`, r.User.ID, now.Add(-24*time.Hour).Unix()).Scan(&today, &last); err != nil {
		return false, err
	}

	if !last.Valid {
		return false, nil
	}

	t := time.Unix(last.Int64, 0)
	interval := max(1, time.Duration(today.Int64/h.Config.BoostThrottleFactor)) * h.Config.BoostThrottleUnit
	return now.Sub(t) < interval, nil
}

func (h *Handler) share(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	postID := "https://" + args[1]

	var noteString string
	if err := r.QueryRow(`select object from notes where id = $1 and public = 1 and author != $2 and not exists (select 1 from shares where note = notes.id and by = $2)`, postID, r.User.ID).Scan(&noteString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Attempted to share non-existing post", "post", postID, "error", err)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch post to share", "post", postID, "error", err)
		w.Error()
		return
	}

	var note ap.Object
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		r.Log.Warn("Failed to unmarshal post to share", "post", postID, "error", err)
		w.Error()
		return
	}

	if throttle, err := h.shouldThrottleBoost(r); err != nil {
		r.Log.Warn("Failed to check if share needs to be throttled", "error", err)
		w.Error()
		return
	} else if throttle {
		r.Log.Warn("User is sharing and unsharing too frequently")
		w.Status(40, "Please wait before sharing")
		return
	}

	if err := outbox.Announce(r.Context, r.Handler.Domain, r.DB, r.User, &note); err != nil {
		r.Log.Warn("Failed to share post", "post", postID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/view/" + args[1])
}
