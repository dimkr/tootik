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
	"crypto/sha256"
	"database/sql"
	"errors"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/text"
	"path/filepath"
)

const maxFollowsPerUser = 150

func follow(w text.Writer, r *request) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	hash := filepath.Base(r.URL.Path)

	followed := ""
	if err := r.QueryRow(`select id from persons where hash = ?`, hash).Scan(&followed); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Cannot follow a non-existing user", "hash", hash)
		w.Status(40, "No such user")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find followed user", "hash", hash, "error", err)
		w.Error()
		return
	}

	var follows int
	if err := r.QueryRow(`select count(*) from follows where follower = ?`, r.User.ID).Scan(&follows); err != nil {
		r.Log.Warn("Failed to count follows", "error", err)
		w.Error()
		return
	}

	if follows >= maxFollowsPerUser {
		w.Status(40, "Following too many users")
		return
	}

	var following int
	if err := r.QueryRow(`select exists (select 1 from follows where follower = ? and followed =?)`, r.User.ID, followed).Scan(&following); err != nil {
		r.Log.Warn("Failed to check if user is already followed", "followed", followed, "error", err)
		w.Error()
		return
	}
	if following == 1 {
		w.Statusf(40, "Already following %s", followed)
		return
	}

	if err := fed.Follow(r.Context, r.User, followed, r.DB); err != nil {
		r.Log.Warn("Failed to follow user", "followed", followed, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/outbox/%x", sha256.Sum256([]byte(followed)))
}
