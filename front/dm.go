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
	"database/sql"
	"encoding/json"
	"errors"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/text"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/dm/[0-9a-f]{64}`)] = dm
}

func dm(w text.Writer, r *request) {
	hash := filepath.Base(r.URL.Path)

	var actorString string
	if err := r.QueryRow(`select actor from persons where hash = ?`, hash).Scan(&actorString); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("User does not exist", "hash", hash)
		w.Status(40, "User does not exist")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find user by hash", "hash", hash, "error", err)
		w.Error()
		return
	}

	actor := ap.Object{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		r.Log.Warn("Failed to unmarshal actor", "hash", hash, "error", err)
		w.Error()
		return
	}

	var following int
	if err := r.QueryRow(`select exists (select 1 from follows where follower = ? and followed = ?)`, actor.ID, r.User.ID).Scan(&following); err != nil {
		r.Log.Warn("Failed to check if user is a follower", "follower", actor.ID, "error", err)
		w.Error()
		return
	} else if following == 0 {
		r.Log.Warn("Cannot DM a user not following", "follower", actor.ID)
		w.Error()
		return
	}

	r.Log.Info("Sending DM to user", "to", actor.ID)

	to := ap.Audience{}
	to.Add(actor.ID)

	cc := ap.Audience{}

	post(w, r, nil, to, cc, "Message")
}
