/*
Copyright 2023 - 2025 Dima Krasner

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
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) say(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	to := ap.Audience{}
	cc := ap.Audience{}

	to.Add(ap.Public)
	cc.Add(r.User.Followers)

	h.post(w, r, nil, nil, "", to, cc, "", func() (string, bool) {
		return readQuery(w, r, "Post content")
	})
}

func (h *Handler) uploadSay(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	to := ap.Audience{}
	cc := ap.Audience{}

	to.Add(ap.Public)
	cc.Add(r.User.Followers)

	h.post(w, r, nil, nil, "", to, cc, "", func() (string, bool) {
		return h.readBody(w, r, args)
	})
}
