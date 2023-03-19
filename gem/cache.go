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
	"bytes"
	"context"
	"database/sql"
	"github.com/dimkr/tootik/data"
	log "github.com/sirupsen/logrus"
	"io"
	"net/url"
	"sync"
	"time"
)

type cacheEntry struct {
	Value   []byte
	Created time.Time
}

var cache sync.Map

func withCache(f func(context.Context, io.Writer, *url.URL, []string, *data.Object, *sql.DB), d time.Duration) func(context.Context, io.Writer, *url.URL, []string, *data.Object, *sql.DB) {
	return func(ctx context.Context, conn io.Writer, requestUrl *url.URL, params []string, user *data.Object, db *sql.DB) {
		key := requestUrl.String()
		now := time.Now()

		entry, cached := cache.Load(key)
		if cached && entry.(cacheEntry).Created.After(now.Add(-d)) {
			log.WithField("key", key).Info("Sending cached response")
			conn.Write(entry.(cacheEntry).Value)
			return
		}

		var buf bytes.Buffer
		f(ctx, &buf, requestUrl, params, user, db)

		resp := buf.Bytes()
		cache.Store(key, cacheEntry{resp, now})
		conn.Write(resp)
	}
}
