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

func init() {
	handlers[regexp.MustCompile(`^/users/unfollow/[0-9a-f]{64}$`)] = withUserMenu(unfollow)
}

func unfollow(w text.Writer, r *request) {
	if r.User == nil {
		w.Redirect("/users")
		return
	}

	hash := filepath.Base(r.URL.Path)

	var followID, followed string
	if err := r.QueryRow(`select follows.id, persons.id from persons join follows on persons.id = follows.followed where persons.hash = ? and follows.follower = ?`, hash, r.User.ID).Scan(&followID, &followed); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.Warn("Cannot undo a non-existing follow", "hash", hash, "error", err)
		w.Status(40, "No such follow")
		return
	} else if err != nil {
		r.Log.Warn("Failed to find followed user", "hash", hash, "error", err)
		w.Error()
		return
	}

	if err := fed.Unfollow(r.Context, r.User, followed, followID, r.DB, r.Log); err != nil {
		r.Log.Warn("Failed undo follow", "followed", followed, "error", err)
		w.Error()
		return
	}

	w.Redirectf("/users/outbox/%x", sha256.Sum256([]byte(followed)))
}
