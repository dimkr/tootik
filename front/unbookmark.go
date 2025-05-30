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

import "github.com/dimkr/tootik/front/text"

func (h *Handler) unbookmark(w text.Writer, r *Request, args ...string) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	postID := "https://" + args[1]

	if _, err := h.DB.ExecContext(r.Context, `delete from bookmarks where note = ? and by = ?`, postID, r.User.ID); err != nil {
		r.Log.Warn("Failed to delete bookmark", "post", postID, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/view/" + args[1])
}
