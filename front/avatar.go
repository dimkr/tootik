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
	"github.com/dimkr/tootik/fed/icon"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/outbox"
	"io"
	"strconv"
	"strings"
	"time"
)

var supportedImageTypes = map[string]struct{}{
	"png":  {},
	"jpeg": {},
	"gif":  {},
}

func (h *Handler) avatar(w text.Writer, r *request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	mimeType := args[1]
	sizeStr := args[2]

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse icon size", "error", err)
		w.Status(40, "Invalid size")
		return
	}

	if size > r.Handler.Config.MaxAvatarSize {
		r.Log.Warn("Image is too big", "size", size)
		w.Status(40, "Image is too big")
		return
	}

	if _, ok := supportedImageTypes[mimeType]; !ok {
		r.Log.Warn("Image type is unsupported", "type", mimeType)
		w.Status(40, "Unsupported image type")
		return
	}

	now := time.Now()

	if (r.User.Updated != nil && now.Sub(r.User.Updated.Time) < h.Config.MinActorEditInterval) || (r.User.Updated == nil && now.Sub(r.User.Published.Time) < h.Config.MinActorEditInterval) {
		r.Log.Warn("Throttled request to set avatar")
		w.Status(40, "Please try again later")
		return
	}

	buf := make([]byte, size)
	_, err = io.ReadFull(r.Body, buf)
	if err != nil {
		r.Log.Warn("Failed to read icon", "error", err)
		w.Error()
		return
	}

	resized, err := icon.Scale(r.Handler.Config, buf)
	if err != nil {
		r.Log.Warn("Failed to read icon", "error", err)
		w.Error()
		return
	}

	tx, err := r.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to set icon", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		r.Context,
		"update persons set actor = json_set(actor, '$.updated', $1) where id = $2",
		now.Format(time.RFC3339Nano),
		r.User.ID,
	); err != nil {
		r.Log.Error("Failed to set icon", "error", err)
		w.Error()
		return
	}

	if _, err := tx.ExecContext(
		r.Context,
		"insert into icons(name, buf) values($1, $2) on conflict(name) do update set buf = $2",
		r.User.PreferredUsername,
		string(resized),
	); err != nil {
		r.Log.Error("Failed to set icon", "error", err)
		w.Error()
		return
	}

	if err := outbox.UpdateActor(r.Context, h.Domain, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to set icon", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to set icon", "error", err)
		w.Error()
		return
	}

	r.Log.Info("Set icon")
	w.Redirect("/users/outbox/" + strings.TrimPrefix(r.User.ID, "https://"))
}
