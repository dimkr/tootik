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
	"slices"
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
	w.c <- slices.Clone(p)
	return len(p), nil
}

func callAndCache(
	r *Request,
	w text.Writer,
	args []string,
	f func(text.Writer, *Request, ...string),
	key string,
	now time.Time,
	cache *sync.Map,
) {
	c := make(chan []byte)

	ctx, cancel := context.WithCancel(context.Background())

	// handle the request without timeout: the listener will close w on timeout
	go func() {
		r2 := *r
		r2.Context = ctx

		w2 := w.Clone(&chanWriter{c})
		f(w2, &r2, args...)

		w2.Flush()
		close(c)
	}()

	var buf bytes.Buffer
	send := true
	for {
		chunk, ok := <-c
		if !ok {
			// always call Flush()
			if err := w.Flush(); err != nil && send {
				r.Log.Warn("Failed to send response", "error", err)
			}

			break
		}

		// send response chunks to the client, until error
		if send {
			if _, err := w.Write(chunk); err != nil {
				r.Log.Warn("Failed to send response", "error", err)
				send = false
			}
		}

		// remember the sent chunk
		buf.Write(chunk)
	}

	// if we're here, w is closed or sending failed
	cancel()

	// append a footer to cached responses
	w2 := w.Clone(&buf)
	w2.Empty()
	w2.Textf("(Cached response generated on %s)", now.Format(time.UnixDate))
	w2.Flush()

	cache.Store(key, cacheEntry{buf.Bytes(), now})
}

func withCache(
	f func(text.Writer, *Request, ...string),
	d time.Duration,
	cache *sync.Map,
	cfg *cfg.Config,
) func(text.Writer, *Request, ...string) {
	return func(w text.Writer, r *Request, args ...string) {
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
