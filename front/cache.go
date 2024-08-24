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

type chanWriter struct {
	c chan<- []byte
}

type cacheEntry struct {
	Value   []byte
	Created time.Time
}

func (w chanWriter) Write(p []byte) (int, error) {
	w.c <- p
	return len(p), nil
}

func callAndCache(r *request, w text.Writer, args []string, f func(text.Writer, *request, ...string), key string, now time.Time, cache *sync.Map) {
	c := make(chan []byte)

	r2 := *r
	r2.Context = context.Background()

	go func() {
		f(w.Clone(&chanWriter{c}), &r2, args...)
		close(c)
	}()

	var buf bytes.Buffer
	for {
		chunk, ok := <-c
		if !ok {
			break
		}
		w.Write(chunk)
		buf.Write(chunk)
	}

	cached := w.Clone(&buf)
	cached.Empty()
	cached.Textf("(Cached response generated on %s)", now.Format(time.UnixDate))

	cache.Store(key, cacheEntry{buf.Bytes(), now})
}

func withCache(f func(text.Writer, *request, ...string), d time.Duration, cache *sync.Map, cfg *cfg.Config) func(text.Writer, *request, ...string) {
	return func(w text.Writer, r *request, args ...string) {
		key := r.URL.String()
		now := time.Now()

		entry, cached := cache.Load(key)
		if !cached {
			r.Log.Info("Generating first response", "key", key)
			callAndCache(r, w, args, f, key, now, cache)
			return
		}

		if entry.(cacheEntry).Created.After(now.Add(-d)) {
			r.Log.Info("Sending cached response", "key", key)
			w.Write(entry.(cacheEntry).Value)
			return
		}

		r.Log.Info("Generating new response", "key", key)
		callAndCache(r, w, args, f, key, now, cache)
	}
}
