/*
Copyright 2023 Dima Krasner

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

func whisper(w text.Writer, r *request) {
	to := ap.Audience{}
	cc := ap.Audience{}

	to.Add(r.User.Followers)

	var followed int
	if err := r.QueryRow(`select exists (select 1 from follows where followed = ? and accepted = 1)`, r.User.ID).Scan(&followed); err != nil {
		r.Log.Error("Failed to check if user is followed", "error", err)
		w.Error()
		return
	}
	if followed == 0 {
		w.Statusf(40, "Users without followers can publish only public posts")
		return
	}

	post(w, r, nil, to, cc, "Post content")
}
