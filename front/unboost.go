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
)

func (h *Handler) unboost(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	postID := "https://" + args[1]

	var boostString string
	if err := r.QueryRow(`select activity from outbox where activity->>'actor' = $1 and activity->>'type' = 'Announce' and activity->>'object' = $2`, r.User.ID, postID).Scan(&boostString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Attempted to unboost non-existing boost", "post", postID, "error", err)
		w.Error()
		return
	} else if err != nil {
		r.Log.Warn("Failed to fetch boost to unboost", "post", postID, "error", err)
		w.Error()
		return
	}

	var boost ap.Activity
	if err := json.Unmarshal([]byte(boostString), &boost); err != nil {
		r.Log.Warn("Failed to unmarshal boost to unboost", "post", postID, "error", err)
		w.Error()
		return
	}

	if throttle, err := h.shouldThrottleBoost(r); err != nil {
		r.Log.Warn("Failed to check if unboost needs to be throttled", "error", err)
		w.Error()
		return
	} else if throttle {
		r.Log.Warn("User is boosting and unboosting too frequently")
		w.Status(40, "Please wait before unboosting")
		return
	}

	if err := outbox.Undo(r.Context, r.Handler.Domain, r.DB, &boost); err != nil {
		r.Log.Warn("Failed to unboost post", "post", postID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/view/" + args[1])
}
