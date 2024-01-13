/*
Copyright 2023, 2024 Dima Krasner

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
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text"
	"path/filepath"
)

func (h *Handler) reply(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	var noteString string
	var group sql.NullString
	if err := r.QueryRow(`select notes.object, notes.groupid from notes join persons on persons.id = notes.author left join (select id, actor from persons where actor->>'type' = 'Group') groups on groups.id = notes.groupid where notes.hash = $1 and (notes.public = 1 or notes.author = $2 or $2 in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = $2)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = $2)) or exists (select 1 from (select persons.id, persons.actor->>'followers' as followers, persons.actor->>'type' as type from persons join follows on follows.followed = persons.id where follows.accepted = 1 and follows.follower = $2) follows where follows.followers in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = follows.followers)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = follows.followers)) or (follows.id = notes.groupid and follows.type = 'Group')))`, hash, r.User.ID).Scan(&noteString, &group); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Post does not exist", "hash", hash)
		w.Status(40, "Post not found")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find post by hash", "hash", hash, "error", err)
		w.Error()
		return
	}

	note := ap.Object{}
	if err := json.Unmarshal([]byte(noteString), &note); err != nil {
		r.Log.Warn("Failed to unmarshal post", "post", note.ID, "error", err)
		w.Error()
		return
	}

	r.Log.Info("Replying to post", "post", note.ID)

	to := ap.Audience{}
	cc := ap.Audience{}

	if note.AttributedTo == r.User.ID {
		to = note.To
		cc = note.CC
	} else if (len(note.To.OrderedMap) == 0 || len(note.To.OrderedMap) == 1 && note.To.Contains(r.User.ID)) && (len(note.CC.OrderedMap) == 0 || len(note.CC.OrderedMap) == 1 && note.CC.Contains(r.User.ID)) {
		to.Add(note.AttributedTo)
	} else if note.IsPublic() {
		to.Add(note.AttributedTo)
		cc.Add(r.User.Followers)
		cc.Add(ap.Public)
	} else if !note.IsPublic() {
		to.Add(note.AttributedTo)
		cc.Add(r.User.Followers)
		note.To.Range(func(id string, _ struct{}) bool {
			cc.Add(id)
			return true
		})
		note.CC.Range(func(id string, _ struct{}) bool {
			cc.Add(id)
			return true
		})
	} else {
		r.Log.Error("Post audience is invalid", "post", note.ID)
		w.Error()
		return
	}

	if group.Valid {
		cc.Add(group.String)
	}

	h.post(w, r, nil, &note, to, cc, "Reply content")
}
