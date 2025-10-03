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
	"time"
	"unicode/utf8"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/front/text/plain"
)

func (h *Handler) bio(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	w.OK()

	w.Title("📜 Bio")

	if len(r.User.Summary) == 0 {
		w.Text("Bio is empty.")
	} else {
		w.Text("Current bio:")
		w.Empty()

		bio, links := getTextAndLinks(r.User.Summary, -1, -1)

		for _, line := range bio {
			w.Quote(line)
		}

		for link, alt := range links.All() {
			if alt == "" {
				w.Link(link, link)
			} else {
				w.Link(link, alt)
			}
		}
	}

	w.Empty()

	w.Link("/users/bio/set", "Set")
	w.Link(fmt.Sprintf("titan://%s/users/bio/upload", h.Domain), "Upload")
}

func (h *Handler) doSetBio(w text.Writer, r *Request, readInput func(text.Writer, *Request) (string, bool)) {
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
		r.Log.Warn("Throttled request to set bio", "can", can)
		w.Statusf(40, "Please wait for %s", time.Until(can).Truncate(time.Second).String())
		return
	}

	bio, ok := readInput(w, r)
	if !ok {
		return
	}

	if utf8.RuneCountInString(bio) > h.Config.MaxBioLength {
		w.Status(40, "Bio is too long")
		return
	}

	r.User.Summary = plain.ToHTML(bio, nil)
	r.User.Updated.Time = now

	if err := h.Inbox.UpdateActor(r.Context, r.User, r.Keys[1]); err != nil {
		r.Log.Error("Failed to update bio", "error", err)
		w.Error()
		return
	}

	if r.URL.Scheme == "titan" {
		w.Redirectf("gemini://%s/users/bio", h.Domain)
	} else {
		w.Redirect("/users/bio")
	}
}

func (h *Handler) setBio(w text.Writer, r *Request, args ...string) {
	h.doSetBio(
		w,
		r,
		func(w text.Writer, r *Request) (string, bool) {
			return readQuery(w, r, "Bio")
		},
	)
}

func (h *Handler) uploadBio(w text.Writer, r *Request, args ...string) {
	h.doSetBio(
		w,
		r,
		func(w text.Writer, r *Request) (string, bool) {
			return h.readBody(w, r, args)
		},
	)
}
