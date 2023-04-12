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
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/text"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/stats$`)] = withCache(withUserMenu(stats), time.Minute*5)
	handlers[regexp.MustCompile(`^/users/stats$`)] = withCache(withUserMenu(stats), time.Minute*5)
}

func stats(w text.Writer, r *request) {
	prefix := fmt.Sprintf("https://%s/%%", cfg.Domain)

	var usersCount, postsCount, federatedPostsCount, lastPost, lastFederatedPost, lastRegister, lastFederatedUser int64
	var queueSize int

	if err := r.QueryRow(`select count(*) from persons where id like ?`, prefix).Scan(&usersCount); err != nil {
		r.Log.WithError(err).Info("Failed to get users count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id like ?`, prefix).Scan(&postsCount); err != nil {
		r.Log.WithError(err).Info("Failed to get posts count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id not like ?`, prefix).Scan(&federatedPostsCount); err != nil {
		r.Log.WithError(err).Info("Failed to get federated posts count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(inserted) from notes where id like ?`, prefix).Scan(&lastPost); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(inserted) from notes where id not like ?`, prefix).Scan(&lastFederatedPost); err != nil {
		r.Log.WithError(err).Info("Failed to get last federated post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(inserted) from persons where id like ?`, prefix).Scan(&lastRegister); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(max(inserted), max(updated)) from persons where id not like ?`, prefix).Scan(&lastFederatedUser); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from deliveries`).Scan(&queueSize); err != nil {
		r.Log.WithError(err).Info("Failed to get delivery queue size")
		w.Error()
		return
	}

	w.OK()

	w.Title("📊 Statistics")

	w.Itemf("Lastest local post: %s", time.Unix(lastPost, 0).Format(time.UnixDate))
	w.Itemf("Lastest federated post: %s", time.Unix(lastFederatedPost, 0).Format(time.UnixDate))
	w.Itemf("Local users: %d", usersCount)
	w.Itemf("Local posts: %d", postsCount)
	w.Itemf("Federated posts: %d", federatedPostsCount)
	w.Itemf("Newest user: %s", time.Unix(lastRegister, 0).Format(time.UnixDate))
	w.Itemf("Latest federated user update: %s", time.Unix(lastFederatedUser, 0).Format(time.UnixDate))
	w.Itemf("Outgoing posts queue size: %d", queueSize)
}
