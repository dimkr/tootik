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
		r.Log.WithField("hash", hash).Warn("User does not exist")
		w.Status(40, "User does not exist")
		return
	} else if err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to find user by hash")
		w.Error()
		return
	}

	actor := ap.Object{}
	if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to unmarshal actor")
		w.Error()
		return
	}

	r.Log.WithField("to", actor.ID).Info("Sending DM to user")

	to := ap.Audience{}
	to.Add(actor.ID)

	cc := ap.Audience{}

	post(w, r, nil, to, cc, "Message")
}
