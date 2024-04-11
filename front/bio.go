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
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
	"github.com/dimkr/tootik/outbox"
	"strings"
	"time"
	"unicode/utf8"
)

func (h *Handler) doBio(w text.Writer, r *request, readContent func(text.Writer, *request) (string, bool)) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	now := time.Now()

	if (r.User.Updated != nil && now.Sub(r.User.Updated.Time) < h.Config.MinActorEditInterval) || (r.User.Updated == nil && now.Sub(r.User.Published.Time) < h.Config.MinActorEditInterval) {
		r.Log.Warn("Throttled request to set summary")
		w.Status(40, "Please try again later")
		return
	}

	summary, ok := readContent(w, r)
	if !ok {
		return
	}

	if utf8.RuneCountInString(summary) > h.Config.MaxBioLength {
		w.Status(40, "Summary is too long")
		return
	}

	tx, err := r.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to update summary", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		r.Context,
		"update persons set actor = json_set(actor, '$.summary', $1, '$.updated', $2) where id = $3",
		plain.ToHTML(summary, nil),
		now.Format(time.RFC3339Nano),
		r.User.ID,
	); err != nil {
		r.Log.Error("Failed to update summary", "error", err)
		w.Error()
		return
	}

	if err := outbox.UpdateActor(r.Context, h.Domain, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to update summary", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to update summary", "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/outbox/" + strings.TrimPrefix(r.User.ID, "https://"))
}

func (h *Handler) bio(w text.Writer, r *request, args ...string) {
	h.doBio(
		w,
		r,
		func(w text.Writer, r *request) (string, bool) {
			return readQuery(w, r, "Bio")
		},
	)
}

func (h *Handler) uploadBio(w text.Writer, r *request, args ...string) {
	h.doBio(
		w,
		r,
		func(w text.Writer, r *request) (string, bool) {
			return readUpload(w, r, args)
		},
	)
}
