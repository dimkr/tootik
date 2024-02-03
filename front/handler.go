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
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front/static"
	"github.com/dimkr/tootik/front/text"
	"log/slog"
	"net/url"
	"regexp"
	"sync"
	"time"
)

// Handler handles frontend (client-to-server) requests.
type Handler struct {
	handlers map[*regexp.Regexp]func(text.Writer, *request, ...string)
	Domain   string
	Config   *cfg.Config
}

var ErrNotRegistered = errors.New("user is not registered")

func serveStaticFile(w text.Writer, r *request, args ...string) {
	w.OK()

	for _, line := range static.Files[r.URL.Path] {
		w.Text(line)
	}
}

// NewHandler returns a new [Handler].
func NewHandler(domain string, closed bool, cfg *cfg.Config) Handler {
	h := Handler{
		handlers: map[*regexp.Regexp]func(text.Writer, *request, ...string){},
		Domain:   domain,
		Config:   cfg,
	}
	var cache sync.Map

	h.handlers[regexp.MustCompile(`^/$`)] = withUserMenu(h.home)

	h.handlers[regexp.MustCompile(`^/users$`)] = withUserMenu(users)
	if closed {
		h.handlers[regexp.MustCompile(`^/users/register$`)] = func(w text.Writer, r *request, args ...string) {
			w.Status(40, "Registration is closed")
		}
	} else {
		h.handlers[regexp.MustCompile(`^/users/register$`)] = h.register
	}

	h.handlers[regexp.MustCompile(`^/users/inbox/([0-9]{4}-[0-9]{2}-[0-9]{2})$`)] = withUserMenu(h.byDate)
	h.handlers[regexp.MustCompile(`^/users/inbox/today$`)] = withUserMenu(h.today)
	h.handlers[regexp.MustCompile(`^/users/inbox/yesterday$`)] = withUserMenu(h.yesterday)
	h.handlers[regexp.MustCompile(`^/users/firehose$`)] = withUserMenu(h.firehose)

	h.handlers[regexp.MustCompile(`^/local$`)] = withCache(withUserMenu(h.local), time.Minute*15, &cache, cfg)
	h.handlers[regexp.MustCompile(`^/users/local$`)] = withCache(withUserMenu(h.local), time.Minute*15, &cache, cfg)

	h.handlers[regexp.MustCompile(`^/federated$`)] = withCache(withUserMenu(h.federated), time.Minute*10, &cache, cfg)
	h.handlers[regexp.MustCompile(`^/users/federated$`)] = withCache(withUserMenu(h.federated), time.Minute*10, &cache, cfg)

	h.handlers[regexp.MustCompile(`^/outbox/(\S+)$`)] = withUserMenu(h.userOutbox)
	h.handlers[regexp.MustCompile(`^/users/outbox/(\S+)$`)] = withUserMenu(h.userOutbox)

	h.handlers[regexp.MustCompile(`^/users/bio$`)] = h.bio
	h.handlers[regexp.MustCompile(`^/users/name$`)] = h.name
	h.handlers[regexp.MustCompile(`^/users/alias$`)] = h.alias
	h.handlers[regexp.MustCompile(`^/users/move$`)] = h.move

	h.handlers[regexp.MustCompile(`^/view/(\S+)$`)] = withUserMenu(h.view)
	h.handlers[regexp.MustCompile(`^/users/view/(\S+)$`)] = withUserMenu(h.view)

	h.handlers[regexp.MustCompile(`^/thread/(\S+)$`)] = withUserMenu(h.thread)
	h.handlers[regexp.MustCompile(`^/users/thread/(\S+)$`)] = withUserMenu(h.thread)

	h.handlers[regexp.MustCompile(`^/users/dm$`)] = h.dm
	h.handlers[regexp.MustCompile(`^/users/whisper$`)] = h.whisper
	h.handlers[regexp.MustCompile(`^/users/say$`)] = h.say

	h.handlers[regexp.MustCompile(`^/users/reply/(\S+)`)] = h.reply

	h.handlers[regexp.MustCompile(`^/users/share/(\S+)`)] = h.share
	h.handlers[regexp.MustCompile(`^/users/unshare/(\S+)`)] = h.unshare

	h.handlers[regexp.MustCompile(`^/users/edit/(\S+)`)] = h.edit
	h.handlers[regexp.MustCompile(`^/users/delete/(\S+)`)] = delete

	h.handlers[regexp.MustCompile(`^/users/resolve$`)] = withUserMenu(h.resolve)

	h.handlers[regexp.MustCompile(`^/users/follow/(\S+)$`)] = withUserMenu(h.follow)
	h.handlers[regexp.MustCompile(`^/users/unfollow/(\S+)$`)] = withUserMenu(h.unfollow)

	h.handlers[regexp.MustCompile(`^/users/follows$`)] = withUserMenu(h.follows)

	h.handlers[regexp.MustCompile(`^/hashtag/([a-zA-Z0-9]+)$`)] = withCache(withUserMenu(h.hashtag), time.Minute*5, &cache, cfg)
	h.handlers[regexp.MustCompile(`^/users/hashtag/([a-zA-Z0-9]+)$`)] = withCache(withUserMenu(h.hashtag), time.Minute*5, &cache, cfg)

	h.handlers[regexp.MustCompile(`^/hashtags$`)] = withCache(withUserMenu(h.hashtags), time.Minute*30, &cache, cfg)
	h.handlers[regexp.MustCompile(`^/users/hashtags$`)] = withCache(withUserMenu(h.hashtags), time.Minute*30, &cache, cfg)

	h.handlers[regexp.MustCompile(`^/search$`)] = withUserMenu(search)
	h.handlers[regexp.MustCompile(`^/users/search$`)] = withUserMenu(search)

	h.handlers[regexp.MustCompile(`^/fts$`)] = withUserMenu(h.fts)
	h.handlers[regexp.MustCompile(`^/users/fts$`)] = withUserMenu(h.fts)

	h.handlers[regexp.MustCompile(`^/stats$`)] = withCache(withUserMenu(h.stats), time.Minute*5, &cache, cfg)
	h.handlers[regexp.MustCompile(`^/users/stats$`)] = withCache(withUserMenu(h.stats), time.Minute*5, &cache, cfg)

	h.handlers[regexp.MustCompile(`^/oops`)] = withUserMenu(oops)
	h.handlers[regexp.MustCompile(`^/users/oops`)] = withUserMenu(oops)

	h.handlers[regexp.MustCompile(`^/robots.txt$`)] = robots

	for path := range static.Files {
		h.handlers[regexp.MustCompile(fmt.Sprintf(`^%s$`, path))] = withUserMenu(serveStaticFile)
	}

	return h
}

// Handle handles a request and writes a response.
func (h *Handler) Handle(ctx context.Context, log *slog.Logger, w text.Writer, reqUrl *url.URL, user *ap.Actor, db *sql.DB, resolver *fed.Resolver, wg *sync.WaitGroup) {
	for re, handler := range h.handlers {
		m := re.FindStringSubmatch(reqUrl.Path)
		if m != nil {
			var l *slog.Logger
			if user == nil {
				l = log.With(slog.Group("request", "path", reqUrl.Path))
			} else {
				l = log.With(slog.Group("request", "path", reqUrl.Path, "user", user.ID))
			}

			handler(
				w,
				&request{
					Context:   ctx,
					Handler:   h,
					URL:       reqUrl,
					User:      user,
					DB:        db,
					Resolver:  resolver,
					WaitGroup: wg,
					Log:       l,
				},
				m...,
			)
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
