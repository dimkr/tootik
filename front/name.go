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
	"crypto/sha256"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/outbox"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	minNameEditInterval = time.Minute * 30
	maxNameLength       = 30
)

func name(w text.Writer, r *request) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := time.Now()

	if (r.User.Updated != nil && now.Sub(r.User.Updated.Time) < minNameEditInterval) || (r.User.Updated == nil && now.Sub(r.User.Published.Time) < minNameEditInterval) {
		r.Log.Warn("Throttled request to set name")
		w.Status(40, "Please try again later")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Display name")
		return
	}

	displayName, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		w.Status(40, "Bad input")
		return
	}

	plainDisplayName, _ := plain.FromHTML(displayName)
	plainDisplayName = strings.Join(strings.Fields(plainDisplayName), " ")
	if plainDisplayName == "" {
		w.Status(10, "Display name")
		return
	}

	if utf8.RuneCountInString(plainDisplayName) > maxNameLength {
		w.Status(40, "Display name is too long")
		return
	}

	tx, err := r.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to update name", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		r.Context,
		"update persons set actor = json_set(actor, '$.name', $1, '$.updated', $2) where id = $3",
		plainDisplayName,
		now.Format(time.RFC3339Nano),
		r.User.ID,
	); err != nil {
		r.Log.Error("Failed to update name", "error", err)
		w.Error()
		return
	}

	if err := outbox.UpdateActor(r.Context, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to update name", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to update name", "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/outbox/%x", sha256.Sum256([]byte(r.User.ID)))
}
