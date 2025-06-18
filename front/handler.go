/*
Copyright 2023 - 2025 Dima Krasner

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
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/static"
	"github.com/dimkr/tootik/front/text"
)

// Handler handles frontend (client-to-server) requests.
type Handler struct {
	handlers map[*regexp.Regexp]func(text.Writer, *Request, ...string)
	Domain   string
	Config   *cfg.Config
	Resolver ap.Resolver
	DB       *sql.DB
}

var (
	ErrNotRegistered = errors.New("user is not registered")
	ErrNotApproved   = errors.New("client certificate is not approved")
)

func serveStaticFile(lines []string, w text.Writer, _ *Request, _ ...string) {
	w.OK()

	for _, line := range lines {
		w.Text(line)
	}
}

// NewHandler returns a new [Handler].
func NewHandler(domain string, closed bool, cfg *cfg.Config, resolver ap.Resolver, db *sql.DB) (Handler, error) {
	h := Handler{
		handlers: map[*regexp.Regexp]func(text.Writer, *Request, ...string){},
		Domain:   domain,
		Config:   cfg,
		Resolver: resolver,
		DB:       db,
	}
	var cache sync.Map

	h.handlers[regexp.MustCompile(`^/$`)] = withUserMenu(h.home)

	h.handlers[regexp.MustCompile(`^/users$`)] = withUserMenu(h.users)
	if closed {
		h.handlers[regexp.MustCompile(`^/users/register$`)] = func(w text.Writer, r *Request, args ...string) {
			w.Status(40, "Registration is closed")
		}
	} else {
		h.handlers[regexp.MustCompile(`^/users/register$`)] = h.register
	}

	h.handlers[regexp.MustCompile(`^/users/mentions$`)] = withUserMenu(h.mentions)

	h.handlers[regexp.MustCompile(`^/local$`)] = withCache(withUserMenu(h.local), time.Minute*15, &cache)
	h.handlers[regexp.MustCompile(`^/users/local$`)] = withCache(withUserMenu(h.local), time.Minute*15, &cache)

	h.handlers[regexp.MustCompile(`^/outbox/(\S+)$`)] = withUserMenu(h.userOutbox)
	h.handlers[regexp.MustCompile(`^/users/outbox/(\S+)$`)] = withUserMenu(h.userOutbox)
	h.handlers[regexp.MustCompile(`^/users/me$`)] = withUserMenu(me)

	h.handlers[regexp.MustCompile(`^/users/upload/avatar;([a-z]+)=([^;]+);([a-z]+)=([^;]+)`)] = h.uploadAvatar
	h.handlers[regexp.MustCompile(`^/users/bio$`)] = h.bio
	h.handlers[regexp.MustCompile(`^/users/metadata$`)] = withUserMenu(h.metadata)
	h.handlers[regexp.MustCompile(`^/users/metadata/add$`)] = h.metadataAdd
	h.handlers[regexp.MustCompile(`^/users/metadata/remove/(\d+)$`)] = h.metadataRemove
	h.handlers[regexp.MustCompile(`^/users/metadata/clear$`)] = h.metadataClear
	h.handlers[regexp.MustCompile(`^/users/upload/bio;([a-z]+)=([^;]+)(?:;([a-z]+)=([^;]+)){0,1}$`)] = h.uploadBio
	h.handlers[regexp.MustCompile(`^/users/name$`)] = h.name
	h.handlers[regexp.MustCompile(`^/users/alias$`)] = h.alias
	h.handlers[regexp.MustCompile(`^/users/move$`)] = h.move
	h.handlers[regexp.MustCompile(`^/users/certificates$`)] = withUserMenu(h.certificates)
	h.handlers[regexp.MustCompile(`^/users/certificates/approve/(\S+)$`)] = withUserMenu(h.approve)
	h.handlers[regexp.MustCompile(`^/users/certificates/revoke/(\S+)$`)] = withUserMenu(h.revoke)
	h.handlers[regexp.MustCompile(`^/users/ttl`)] = withUserMenu(h.ttl)
	h.handlers[regexp.MustCompile(`^/users/export$`)] = h.export
	h.handlers[regexp.MustCompile(`^/users/approve/(\S+)$`)] = withUserMenu(h.approve)

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

	h.handlers[regexp.MustCompile(`^/users/bookmark/(\S+)`)] = h.bookmark
	h.handlers[regexp.MustCompile(`^/users/unbookmark/(\S+)`)] = h.unbookmark
	h.handlers[regexp.MustCompile(`^/users/bookmarks$`)] = withUserMenu(h.bookmarks)

	h.handlers[regexp.MustCompile(`^/users/edit/(\S+)`)] = h.edit
	h.handlers[regexp.MustCompile(`^/users/delete/(\S+)`)] = h.delete

	h.handlers[regexp.MustCompile(`^/users/upload/dm;([a-z]+)=([^;]+)(?:;([a-z]+)=([^;]+)){0,1}$`)] = h.uploadDM
	h.handlers[regexp.MustCompile(`^/users/upload/whisper;([a-z]+)=([^;]+)(?:;([a-z]+)=([^;]+)){0,1}$`)] = h.uploadWhisper
	h.handlers[regexp.MustCompile(`^/users/upload/say;([a-z]+)=([^;]+)(?:;([a-z]+)=([^;]+)){0,1}$`)] = h.uploadSay
	h.handlers[regexp.MustCompile(`^/users/upload/edit/([^;]+);([a-z]+)=([^;]+)(?:;([a-z]+)=([^;]+)){0,1}$`)] = h.editUpload
	h.handlers[regexp.MustCompile(`^/users/upload/reply/([^;]+);([a-z]+)=([^;]+)(?:;([a-z]+)=([^;]+)){0,1}$`)] = h.replyUpload

	h.handlers[regexp.MustCompile(`^/users/resolve$`)] = withUserMenu(h.resolve)

	h.handlers[regexp.MustCompile(`^/users/follow/(\S+)$`)] = withUserMenu(h.follow)
	h.handlers[regexp.MustCompile(`^/users/unfollow/(\S+)$`)] = withUserMenu(h.unfollow)

	h.handlers[regexp.MustCompile(`^/users/follows$`)] = withUserMenu(h.follows)
	h.handlers[regexp.MustCompile(`^/users/followers$`)] = withUserMenu(h.followers)
	h.handlers[regexp.MustCompile(`^/users/followers/accept/(\S+)$`)] = withUserMenu(h.accept)
	h.handlers[regexp.MustCompile(`^/users/followers/reject/(\S+)$`)] = withUserMenu(h.reject)

	h.handlers[regexp.MustCompile(`^/communities$`)] = withUserMenu(h.communities)
	h.handlers[regexp.MustCompile(`^/users/communities$`)] = withUserMenu(h.communities)

	h.handlers[regexp.MustCompile(`^/hashtag/([a-zA-Z0-9]+)$`)] = withCache(withUserMenu(h.hashtag), time.Minute*5, &cache)
	h.handlers[regexp.MustCompile(`^/users/hashtag/([a-zA-Z0-9]+)$`)] = withCache(withUserMenu(h.hashtag), time.Minute*5, &cache)

	h.handlers[regexp.MustCompile(`^/hashtags$`)] = withCache(withUserMenu(h.hashtags), time.Minute*30, &cache)
	h.handlers[regexp.MustCompile(`^/users/hashtags$`)] = withCache(withUserMenu(h.hashtags), time.Minute*30, &cache)

	h.handlers[regexp.MustCompile(`^/search$`)] = withUserMenu(search)
	h.handlers[regexp.MustCompile(`^/users/search$`)] = withUserMenu(search)

	h.handlers[regexp.MustCompile(`^/fts$`)] = withUserMenu(h.fts)
	h.handlers[regexp.MustCompile(`^/users/fts$`)] = withUserMenu(h.fts)

	h.handlers[regexp.MustCompile(`^/status$`)] = withCache(withUserMenu(h.status), time.Minute*5, &cache)
	h.handlers[regexp.MustCompile(`^/users/status$`)] = withCache(withUserMenu(h.status), time.Minute*5, &cache)

	h.handlers[regexp.MustCompile(`^/oops`)] = withUserMenu(oops)
	h.handlers[regexp.MustCompile(`^/users/oops`)] = withUserMenu(oops)

	h.handlers[regexp.MustCompile(`^/robots.txt$`)] = robots

	files, err := static.Format(domain, cfg)
	if err != nil {
		return h, err
	}

	for path, lines := range files {
		h.handlers[regexp.MustCompile(fmt.Sprintf(`^%s$`, path))] = withUserMenu(func(w text.Writer, r *Request, args ...string) {
			serveStaticFile(lines, w, r, args...)
		})
	}

	return h, nil
}

// Handle handles a request and writes a response.
func (h *Handler) Handle(r *Request, w text.Writer) {
	for re, handler := range h.handlers {
		m := re.FindStringSubmatch(r.URL.Path)
		if m != nil {
			handler(w, r, m...)
			return
		}
	}

	r.Log.Warn("Received an invalid request")

	if r.URL.Scheme == "titan" && r.User == nil {
		w.Redirectf("gemini://%s/oops", h.Domain)
	} else if r.URL.Scheme == "titan" && r.User != nil {
		w.Redirectf("gemini://%s/users/oops", h.Domain)
	} else if r.User == nil {
		w.Redirect("/oops")
	} else {
		w.Redirect("/users/oops")
	}
}
