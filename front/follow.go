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
	"regexp"
)

const maxFollowsPerUser = 150

func init() {
	handlers[regexp.MustCompile(`^/users/follow/[0-9a-f]{64}$`)] = withUserMenu(follow)
}

func follow(w text.Writer, r *request) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	hash := filepath.Base(r.URL.Path)

	followed := ""
	if err := r.QueryRow(`select id from persons where hash = ?`, hash).Scan(&followed); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.WithField("hash", hash).WithError(err).Warn("Cannot follow a non-existing user")
		w.Status(40, "No such user")
		return
	} else if err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to find followed user")
		w.Error()
		return
	}

	var follows int
	if err := r.QueryRow(`select count(*) from follows where follower = ?`, r.User.ID).Scan(&follows); err != nil {
		r.Log.WithError(err).Warn("Failed to count follows")
		w.Error()
		return
	}

	if follows >= maxFollowsPerUser {
		w.Status(40, "Following too many users")
		return
	}

	if err := fed.Follow(r.Context, r.User, followed, r.DB); err != nil {
		r.Log.WithField("followed", followed).WithError(err).Warn("Failed to follow user")
		w.Error()
		return
	}

	w.Redirectf("/users/outbox/%x", sha256.Sum256([]byte(followed)))
}
