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
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"io"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/stats$`)] = withCache(withUserMenu(stats), time.Minute*5)
	handlers[regexp.MustCompile(`^/users/stats$`)] = withCache(withUserMenu(stats), time.Minute*5)
}

func stats(w io.Writer, r *request) {
	prefix := fmt.Sprintf("https://%s/%%", cfg.Domain)

	var usersCount, postsCount, federatedPostsCount, lastPost, lastFederatedPost, lastRegister, lastFederatedUser int64
	var queueSize int

	if err := r.QueryRow(`select count(*) from persons where id like ?`, prefix).Scan(&usersCount); err != nil {
		r.Log.WithError(err).Info("Failed to get users count")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id like ?`, prefix).Scan(&postsCount); err != nil {
		r.Log.WithError(err).Info("Failed to get posts count")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id not like ?`, prefix).Scan(&federatedPostsCount); err != nil {
		r.Log.WithError(err).Info("Failed to get federated posts count")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := r.QueryRow(`select max(inserted) from notes where id like ?`, prefix).Scan(&lastPost); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := r.QueryRow(`select max(inserted) from notes where id not like ?`, prefix).Scan(&lastFederatedPost); err != nil {
		r.Log.WithError(err).Info("Failed to get last federated post time")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := r.QueryRow(`select max(inserted) from persons where id like ?`, prefix).Scan(&lastRegister); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := r.QueryRow(`select max(max(inserted), max(updated)) from persons where id not like ?`, prefix).Scan(&lastFederatedUser); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	if err := r.QueryRow(`select count(*) from deliveries`).Scan(&queueSize); err != nil {
		r.Log.WithError(err).Info("Failed to get delivery queue size")
		w.Write([]byte("40 Error\r\n"))
		return
	}

	w.Write([]byte("20 text/gemini\r\n"))

	w.Write([]byte("# 📊 Statistics\n\n"))

	fmt.Fprintf(w, "* Lastest local post: %s\n", time.Unix(lastPost, 0).Format(time.UnixDate))
	fmt.Fprintf(w, "* Lastest federated post: %s\n", time.Unix(lastFederatedPost, 0).Format(time.UnixDate))
	fmt.Fprintf(w, "* Local users: %d\n", usersCount)
	fmt.Fprintf(w, "* Local posts: %d\n", postsCount)
	fmt.Fprintf(w, "* Federated posts: %d\n", federatedPostsCount)
	fmt.Fprintf(w, "* Newest user: %s\n", time.Unix(lastRegister, 0).Format(time.UnixDate))
	fmt.Fprintf(w, "* Latest federated user update: %s\n", time.Unix(lastFederatedUser, 0).Format(time.UnixDate))
	fmt.Fprintf(w, "* Outgoing posts queue size: %d\n", queueSize)
}
