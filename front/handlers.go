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
	"github.com/dimkr/tootik/logger"
	"github.com/dimkr/tootik/text"
	log "github.com/sirupsen/logrus"
	"net/url"
	"regexp"
	"sync"
)

var (
	handlers         = map[*regexp.Regexp]func(text.Writer, *request){}
	ErrNotRegistered = errors.New("User is not registered")
)

func Handle(ctx context.Context, w text.Writer, reqUrl *url.URL, user *ap.Actor, db *sql.DB, wg *sync.WaitGroup) {
	for re, handler := range handlers {
		if re.MatchString(reqUrl.Path) {
			logFields := log.Fields{"path": reqUrl.Path}
			if user != nil {
				logFields["user"] = user.ID
			}

			handler(w, &request{
				Context:    ctx,
				URL:        reqUrl,
				User:       user,
				AuthPrefix: "/users",
				DB:         db,
				WaitGroup:  wg,
				Log:        logger.New(logFields),
			})
			return
		}
	}

	log.WithField("path", reqUrl.Path).Warnf("Received an invalid request")

	if user == nil {
		w.Redirect("/oops")
	} else {
		w.Redirect("/users/oops")
	}
}
