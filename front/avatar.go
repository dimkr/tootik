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
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/icon"
	"github.com/dimkr/tootik/outbox"
)

var supportedImageTypes = map[string]struct{}{
	"image/png":  {},
	"image/jpeg": {},
	"image/gif":  {},
}

func (h *Handler) uploadAvatar(w text.Writer, r *Request, args ...string) {
	if r.User == nil || r.Body == nil {
		w.Redirectf("gemini://%s/users/oops", h.Domain)
		return
	}

	var sizeStr, mimeType string
	if args[1] == "size" && args[3] == "mime" {
		sizeStr = args[2]
		mimeType = args[4]
	} else if args[1] == "mime" && args[3] == "size" {
		sizeStr = args[4]
		mimeType = args[2]
	} else {
		r.Log.Warn("Invalid parameters")
		w.Error()
		return
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		r.Log.Warn("Failed to parse avatar size", "error", err)
		w.Status(40, "Invalid size")
		return
	}

	if size > h.Config.MaxAvatarSize {
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

	can := r.User.Published.Time.Add(h.Config.MinActorEditInterval)
	if r.User.Updated != (ap.Time{}) {
		can = r.User.Updated.Time.Add(h.Config.MinActorEditInterval)
	}
	if now.Before(can) {
		r.Log.Warn("Throttled request to set avatar", "can", can)
		w.Statusf(40, "Please wait for %s", time.Until(can).Truncate(time.Second).String())
		return
	}

	buf := make([]byte, size)
	n, err := io.ReadFull(r.Body, buf)
	if err != nil {
		r.Log.Warn("Failed to read avatar", "error", err)
		w.Error()
		return
	}

	if int64(n) != size {
		r.Log.Warn("Avatar is truncated")
		w.Error()
		return
	}

	resized, err := icon.Scale(h.Config, buf)
	if err != nil {
		r.Log.Warn("Failed to read avatar", "error", err)
		w.Error()
		return
	}

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to set avatar", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		r.Context,
		"update persons set actor = jsonb_set(actor, '$.icon.url', $1, '$.icon[0].url', $1, '$.updated', $2) where id = $3",
		// we add fragment because some servers cache the image until the URL changes
		fmt.Sprintf("https://%s/icon/%s%s#%d", h.Domain, r.User.PreferredUsername, icon.FileNameExtension, now.UnixNano()),
		now.Format(time.RFC3339Nano),
		r.User.ID,
	); err != nil {
		r.Log.Error("Failed to set avatar", "error", err)
		w.Error()
		return
	}

	if _, err := tx.ExecContext(
		r.Context,
		"insert into icons(name, buf) values($1, $2) on conflict(name) do update set buf = $2",
		r.User.PreferredUsername,
		string(resized),
	); err != nil {
		r.Log.Error("Failed to set avatar", "error", err)
		w.Error()
		return
	}

	if err := outbox.UpdateActor(r.Context, h.Domain, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to set avatar", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to set avatar", "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/avatar")
}

func (h *Handler) avatar(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	w.OK()

	w.Title("🗿 Avatar")

	if len(r.User.Icon) == 0 || r.User.Icon[0].URL == "" {
		w.Text("Avatar is not set.")
	} else {
		w.Link(r.User.Icon[0].URL, "Current avatar")
	}

	w.Empty()

	w.Link(fmt.Sprintf("titan://%s/users/avatar/upload", h.Domain), "Upload")
}
