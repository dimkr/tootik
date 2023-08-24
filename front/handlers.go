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
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/text"
	"log/slog"
	"net/url"
	"regexp"
	"sync"
)

var (
	handlers         = map[*regexp.Regexp]func(text.Writer, *request){}
	ErrNotRegistered = errors.New("User is not registered")
)

func Handle(ctx context.Context, log *slog.Logger, w text.Writer, reqUrl *url.URL, user *ap.Actor, db *sql.DB, resolver *fed.Resolver, wg *sync.WaitGroup) {
	for re, handler := range handlers {
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
