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
	"bytes"
	"context"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/text"
	"sync"
	"time"
)

type cacheEntry struct {
	Value   []byte
	Created time.Time
}

func callAndCache(r *request, w text.Writer, args []string, f func(text.Writer, *request, ...string), key string, now time.Time, cache *sync.Map) []byte {
	var buf bytes.Buffer
	w2 := w.Clone(&buf)

	r2 := *r
	r2.Context = context.Background()

	f(w2, &r2, args...)

	resp := buf.Bytes()

	raw := make([]byte, len(resp))
	copy(raw, resp)

	w2.Empty()
	w2.Textf("(Cached response generated on %s)", now.Format(time.UnixDate))

	cache.Store(key, cacheEntry{buf.Bytes(), now})
	return raw
}

func withCache(f func(text.Writer, *request, ...string), d time.Duration, cache *sync.Map, cfg *cfg.Config) func(text.Writer, *request, ...string) {
	return func(w text.Writer, r *request, args ...string) {
		key := r.URL.String()
		now := time.Now()

		entry, cached := cache.Load(key)
		if !cached {
			r.Log.Info("Generating first response", "key", key)
			w.Write(callAndCache(r, w, args, f, key, now, cache))
			return
		}

		if entry.(cacheEntry).Created.After(now.Add(-d)) {
			r.Log.Info("Sending cached response", "key", key)
			w.Write(entry.(cacheEntry).Value)
			return
		}

		update := make(chan []byte, 1)
		timer := time.NewTimer(cfg.CacheUpdateTimeout)
		defer timer.Stop()

		r.WaitGroup.Add(1)
		go func() {
			r.Log.Info("Generating new response", "key", key)
			update <- callAndCache(r, w, args, f, key, now, cache)
			r.WaitGroup.Done()
		}()

		select {
		case resp := <-update:
			w.Write(resp)

		case <-timer.C:
			r.Log.Warn("Timeout, sending old cached response", "key", key)
			w.Write(entry.(cacheEntry).Value)
		}
	}
}
