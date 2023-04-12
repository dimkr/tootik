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

package gem

import (
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/fed"
	"io"
	"path/filepath"
	"regexp"
)

func init() {
	handlers[regexp.MustCompile(`^/users/follow/[0-9a-f]{64}$`)] = withUserMenu(follow)
}

func follow(w io.Writer, r *request) {
	if r.User == nil {
		w.Write([]byte("30 /users\r\n"))
		return
	}

	hash := filepath.Base(r.URL.Path)

	followed := ""
	if err := r.QueryRow(`select id from persons where hash = ?`, hash).Scan(&followed); err != nil && errors.Is(err, sql.ErrNoRows) {
		r.Log.WithField("hash", hash).WithError(err).Warn("Cannot follow a non-existing user")
		w.Write([]byte("40 No such user\r\n"))
		return
	} else if err != nil {
		r.Log.WithField("hash", hash).WithError(err).Warn("Failed to find followed user")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := fed.Follow(r.Context, r.User, followed, r.DB); err != nil {
		r.Log.WithField("followed", followed).WithError(err).Warn("Failed to follow user")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	fmt.Fprintf(w, "30 /users/outbox/%x\r\n", sha256.Sum256([]byte(followed)))
}
