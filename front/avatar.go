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
	"log/slog"
	"strconv"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/icon"
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
		slog.WarnContext(r.Context, "Invalid parameters")
		w.Error()
		return
	}

	size, err := strconv.ParseInt(sizeStr, 10, 64)
	if err != nil {
		slog.WarnContext(r.Context, "Failed to parse avatar size", "error", err)
		w.Status(40, "Invalid size")
		return
	}

	if size > h.Config.MaxAvatarSize {
		slog.WarnContext(r.Context, "Image is too big", "size", size)
		w.Status(40, "Image is too big")
		return
	}

	if _, ok := supportedImageTypes[mimeType]; !ok {
		slog.WarnContext(r.Context, "Image type is unsupported", "type", mimeType)
		w.Status(40, "Unsupported image type")
		return
	}

	now := time.Now()

	can := r.User.Published.Time.Add(h.Config.MinActorEditInterval)
	if r.User.Updated != (ap.Time{}) {
		can = r.User.Updated.Time.Add(h.Config.MinActorEditInterval)
	}
	if now.Before(can) {
		slog.WarnContext(r.Context, "Throttled request to set avatar", "can", can)
		w.Statusf(40, "Please wait for %s", time.Until(can).Truncate(time.Second).String())
		return
	}

	buf := make([]byte, size)
	n, err := io.ReadFull(r.Body, buf)
	if err != nil {
		slog.WarnContext(r.Context, "Failed to read avatar", "error", err)
		w.Error()
		return
	}

	if int64(n) != size {
		slog.WarnContext(r.Context, "Avatar is truncated")
		w.Error()
		return
	}

	resized, err := icon.Scale(h.Config, buf)
	if err != nil {
		slog.WarnContext(r.Context, "Failed to read avatar", "error", err)
		w.Error()
		return
	}

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		slog.WarnContext(r.Context, "Failed to set avatar", "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		r.Context,
		"insert into icons(name, buf) values($1, $2) on conflict(name) do update set buf = $2",
		r.User.PreferredUsername,
		string(resized),
	); err != nil {
		slog.ErrorContext(r.Context, "Failed to set avatar", "error", err)
		w.Error()
		return
	}

	// we add fragment because some servers cache the image until the URL changes
	r.User.Icon = append(r.User.Icon, ap.Attachment{
		URL: fmt.Sprintf("https://%s/icon/%s%s#%d", h.Domain, r.User.PreferredUsername, icon.FileNameExtension, now.UnixNano()),
	})
	r.User.Updated.Time = now

	if err := h.Inbox.UpdateActorTx(r.Context, tx, r.User, r.Keys[1]); err != nil {
		slog.ErrorContext(r.Context, "Failed to set avatar", "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		slog.ErrorContext(r.Context, "Failed to set avatar", "error", err)
		w.Error()
		return
	}

	w.Redirectf("gemini://%s/users/avatar", h.Domain)
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
