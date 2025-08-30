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
	"net/url"
	"regexp"
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
		r.Log.Warn("Throttled request to add metadata field", "can", can)
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
		r.Log.Warn("Failed to parse metadata field", "raw", r.URL.RawQuery, "error", err)
		w.Status(40, "Bad input")
		return
	}

	m := metadataRegex.FindStringSubmatch(s)
	if m == nil {
		r.Log.Warn("Invalid metadata field", "field", s)
		w.Status(40, "Bad input")
		return
	}

	attachment := ap.Attachment{
		Type: ap.PropertyValue,
		Name: html.EscapeString(m[1]),
		Val:  plain.ToHTML(m[2], nil),
	}

	r.Log.Info("Adding metadata field", "name", attachment.Name)

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to add metadata field", "name", attachment.Name, "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if res, err := tx.ExecContext(
		r.Context,
		`
		update persons
		set actor = jsonb_set(jsonb_insert(actor, '$.attachment[#]', json($1)), '$.updated', $2)
		where
			id = $3 and
			coalesce(json_array_length(actor->>'$.attachment'), 0) < $4 and
			not exists (select 1 from json_each(actor->'$.attachment') where value->>'$.name' = $5)
		`,
		&attachment,
		now.Format(time.RFC3339Nano),
		r.User.ID,
		h.Config.MaxMetadataFields,
		attachment.Name,
	); err != nil {
		r.Log.Error("Failed to add metadata field", "name", attachment.Name, "error", err)
		w.Error()
		return
	} else if one, err := res.RowsAffected(); err != nil {
		r.Log.Error("Failed to add metadata field", "name", attachment.Name, "error", err)
		w.Error()
		return
	} else if one < 1 {
		r.Log.Error("Cannot add metadata field", "name", attachment.Name)
		w.Status(40, "Cannot add metadata field")
		return
	}

	if err := h.Inbox.UpdateActor(r.Context, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to add metadata field", "name", attachment.Name, "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to add metadata field", "name", attachment.Name, "error", err)
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
		r.Log.Warn("Failed to parse metadata field key", "raw", r.URL.RawQuery, "error", err)
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

	r.Log.Warn("Metadata field key does not exist", "raw", r.URL.RawQuery)
	w.Status(40, "Field does not exist")
	return

found:
	r.Log.Info("Removing metadata field", "key", key)

	tx, err := h.DB.BeginTx(r.Context, nil)
	if err != nil {
		r.Log.Warn("Failed to remove metadata field", "key", key, "error", err)
		w.Error()
		return
	}
	defer tx.Rollback()

	if res, err := tx.ExecContext(
		r.Context,
		`
		update persons
		set actor = jsonb_set(jsonb_remove(actor, '$.attachment[' || $1 || ']'), '$.updated', $2)
		where
			id = $3 and
			json_extract(actor, '$.attachment[' || $1 || '].name') = $4
		`,
		id,
		time.Now().Format(time.RFC3339Nano),
		r.User.ID,
		key,
	); err != nil {
		r.Log.Error("Failed to remove metadata field", "key", key, "id", id, "error", err)
		w.Error()
		return
	} else if one, err := res.RowsAffected(); err != nil {
		r.Log.Error("Failed to remove metadata field", "key", key, "id", id, "error", err)
		w.Error()
		return
	} else if one < 1 {
		r.Log.Error("Failed to remove metadata field", "key", key, "id", id)
		w.Status(40, "Field does not exist")
		return
	}

	if err := h.Inbox.UpdateActor(r.Context, tx, r.User.ID); err != nil {
		r.Log.Error("Failed to remove metadata field", "key", key, "id", id, "error", err)
		w.Error()
		return
	}

	if err := tx.Commit(); err != nil {
		r.Log.Error("Failed to remove metadata field", "key", key, "id", id, "error", err)
		w.Error()
		return
	}

	w.Redirect("/users/metadata")
}
