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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front/static"
	"github.com/dimkr/tootik/front/text"
	"log/slog"
	"net/url"
	"regexp"
	"sync"
	"time"
)

type Handler map[*regexp.Regexp]func(text.Writer, *request)

var ErrNotRegistered = errors.New("user is not registered")

func serveStaticFile(w text.Writer, r *request) {
	w.OK()

	for _, line := range static.Files[r.URL.Path] {
		w.Text(line)
	}
}

func NewHandler(closed bool) Handler {
	h := Handler{}
	var cache sync.Map

	h[regexp.MustCompile(`^/$`)] = withUserMenu(home)

	h[regexp.MustCompile(`^/users$`)] = withUserMenu(users)
	if closed {
		h[regexp.MustCompile(`^/users/register$`)] = func(w text.Writer, r *request) {
			w.Status(40, "Registration is closed")
		}
	} else {
		h[regexp.MustCompile(`^/users/register$`)] = register
	}

	h[regexp.MustCompile(`^/users/inbox/[0-9]{4}-[0-9]{2}-[0-9]{2}$`)] = withUserMenu(byDate)
	h[regexp.MustCompile(`^/users/inbox/today$`)] = withUserMenu(today)
	h[regexp.MustCompile(`^/users/inbox/yesterday$`)] = withUserMenu(yesterday)

	h[regexp.MustCompile(`^/local$`)] = withCache(withUserMenu(local), time.Minute*15, &cache)
	h[regexp.MustCompile(`^/users/local$`)] = withCache(withUserMenu(local), time.Minute*15, &cache)

	h[regexp.MustCompile(`^/federated$`)] = withCache(withUserMenu(federated), time.Minute*10, &cache)
	h[regexp.MustCompile(`^/users/federated$`)] = withCache(withUserMenu(federated), time.Minute*10, &cache)

	h[regexp.MustCompile(`^/outbox/[0-9a-f]{64}$`)] = withUserMenu(userOutbox)
	h[regexp.MustCompile(`^/users/outbox/[0-9a-f]{64}$`)] = withUserMenu(userOutbox)

	h[regexp.MustCompile(`^/view/[0-9a-f]{64}$`)] = withUserMenu(view)
	h[regexp.MustCompile(`^/users/view/[0-9a-f]{64}$`)] = withUserMenu(view)

	h[regexp.MustCompile(`^/thread/[0-9a-f]{64}$`)] = withUserMenu(thread)
	h[regexp.MustCompile(`^/users/thread/[0-9a-f]{64}$`)] = withUserMenu(thread)

	h[regexp.MustCompile(`^/users/whisper$`)] = whisper
	h[regexp.MustCompile(`^/users/say$`)] = say
	h[regexp.MustCompile(`^/users/dm/[0-9a-f]{64}`)] = dm

	h[regexp.MustCompile(`^/users/reply/[0-9a-f]{64}`)] = reply

	h[regexp.MustCompile(`^/users/edit/[0-9a-f]{64}`)] = edit
	h[regexp.MustCompile(`^/users/delete/[0-9a-f]{64}`)] = delete

	h[regexp.MustCompile(`^/users/resolve$`)] = withUserMenu(resolve)

	h[regexp.MustCompile(`^/users/follow/[0-9a-f]{64}$`)] = withUserMenu(follow)
	h[regexp.MustCompile(`^/users/unfollow/[0-9a-f]{64}$`)] = withUserMenu(unfollow)

	h[regexp.MustCompile(`^/users/follows$`)] = withUserMenu(follows)

	h[regexp.MustCompile(`^/hashtag/[a-zA-Z0-9]+$`)] = withCache(withUserMenu(hashtag), time.Minute*5, &cache)
	h[regexp.MustCompile(`^/users/hashtag/[a-zA-Z0-9]+$`)] = withCache(withUserMenu(hashtag), time.Minute*5, &cache)

	h[regexp.MustCompile(`^/hashtags$`)] = withCache(withUserMenu(hashtags), time.Minute*30, &cache)
	h[regexp.MustCompile(`^/users/hashtags$`)] = withCache(withUserMenu(hashtags), time.Minute*30, &cache)

	h[regexp.MustCompile(`^/search$`)] = withUserMenu(search)
	h[regexp.MustCompile(`^/users/search$`)] = withUserMenu(search)

	h[regexp.MustCompile(`^/stats$`)] = withCache(withUserMenu(stats), time.Minute*5, &cache)
	h[regexp.MustCompile(`^/users/stats$`)] = withCache(withUserMenu(stats), time.Minute*5, &cache)

	h[regexp.MustCompile(`^/oops`)] = withUserMenu(oops)
	h[regexp.MustCompile(`^/users/oops`)] = withUserMenu(oops)

	h[regexp.MustCompile(`^/robots.txt$`)] = robots

	for path := range static.Files {
		h[regexp.MustCompile(fmt.Sprintf(`^%s$`, path))] = withUserMenu(serveStaticFile)
	}

	return h
}

func (h Handler) Handle(ctx context.Context, log *slog.Logger, w text.Writer, reqUrl *url.URL, user *ap.Actor, db *sql.DB, resolver *fed.Resolver, wg *sync.WaitGroup) {
	for re, handler := range h {
		if re.MatchString(reqUrl.Path) {
			var l *slog.Logger
			if user == nil {
				l = log.With(slog.Group("request", "path", reqUrl.Path))
			} else {
				l = log.With(slog.Group("request", "path", reqUrl.Path, "user", user.ID))
			}

			handler(w, &request{
				Context:   ctx,
				URL:       reqUrl,
				User:      user,
				DB:        db,
				Resolver:  resolver,
				WaitGroup: wg,
				Log:       l,
			})
			return
		}
	}

	log.Warn("Received an invalid request", "path", reqUrl.Path)

	if user == nil {
		w.Redirect("/oops")
	} else {
		w.Redirect("/users/oops")
	}
}
