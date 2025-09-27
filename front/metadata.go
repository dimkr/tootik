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
	"html"
	"log/slog"
	"net/url"
	"regexp"
	"slices"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
)

var metadataRegex = regexp.MustCompile(`^([^\p{Cc}\p{Cs}\s=\r\n]{1,16}(?: *[^\p{Cc}\p{Cs}\s=\r\n]{1,16}){0,3})=([^\p{Cc}\p{Cs}\r\n]{1,64})$`)

func (h *Handler) metadata(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	w.OK()

	w.Title("💳 Metadata")

	if len(r.User.Attachment) == 0 {
		w.Text("No metadata fields are defined.")
	} else {
		for i, field := range r.User.Attachment {
			if field.Type != ap.PropertyValue || field.Name == "" {
				continue
			}

			if i > 0 {
				w.Empty()
			}

			writeMetadataField(field, w)
			w.Link("/users/metadata/remove?"+url.QueryEscape(field.Name), "➖ Remove")
		}
	}

	if len(r.User.Attachment) < h.Config.MaxMetadataFields {
		w.Empty()
		w.Link("/users/metadata/add", "➕ Add")
	}
}

func (h *Handler) metadataAdd(w text.Writer, r *Request, args ...string) {
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
		slog.WarnContext(r.Context, "Throttled request to add metadata field", "can", can)
		w.Statusf(40, "Please wait for %s", time.Until(can).Truncate(time.Second).String())
		return
	}

	if len(r.User.Attachment) >= h.Config.MaxMetadataFields {
		w.Status(40, "Reached the maximum number of metadata fields")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Metadata field (key=value)")
		return
	}

	s, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		slog.WarnContext(r.Context, "Failed to parse metadata field", "raw", r.URL.RawQuery, "error", err)
		w.Status(40, "Bad input")
		return
	}

	m := metadataRegex.FindStringSubmatch(s)
	if m == nil {
		slog.WarnContext(r.Context, "Invalid metadata field", "field", s)
		w.Status(40, "Bad input")
		return
	}

	name := html.EscapeString(m[1])

	for _, field := range r.User.Attachment {
		if field.Name == name {
			slog.ErrorContext(r.Context, "Cannot add metadata field", "name", field.Name)
			w.Status(40, "Cannot add metadata field")
			return
		}
	}

	attachment := ap.Attachment{
		Type: ap.PropertyValue,
		Name: name,
		Val:  plain.ToHTML(m[2], nil),
	}

	slog.InfoContext(r.Context, "Adding metadata field", "name", attachment.Name)

	r.User.Attachment = append(r.User.Attachment, attachment)
	r.User.Updated.Time = now

	if err := h.Inbox.UpdateActor(r.Context, r.User, r.Keys[1]); err != nil {
		slog.ErrorContext(r.Context, "Failed to add metadata field", "name", attachment.Name, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/metadata")
}

func (h *Handler) metadataRemove(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	if r.URL.RawQuery == "" {
		w.Status(10, "Metadata field (key)")
		return
	}

	key, err := url.QueryUnescape(r.URL.RawQuery)
	if err != nil {
		slog.WarnContext(r.Context, "Failed to parse metadata field key", "raw", r.URL.RawQuery, "error", err)
		w.Status(40, "Bad input")
		return
	}

	id := 0
	for i, field := range r.User.Attachment {
		if field.Name == key {
			id = i
			goto found
		}
	}

	slog.WarnContext(r.Context, "Metadata field key does not exist", "raw", r.URL.RawQuery)
	w.Status(40, "Field does not exist")
	return

found:
	slog.InfoContext(r.Context, "Removing metadata field", "key", key)

	r.User.Attachment = slices.Delete(r.User.Attachment, id, id+1)
	r.User.Updated.Time = time.Now()

	if err := h.Inbox.UpdateActor(r.Context, r.User, r.Keys[1]); err != nil {
		slog.ErrorContext(r.Context, "Failed to remove metadata field", "key", key, "id", id, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/metadata")
}
