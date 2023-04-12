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
	"fmt"
	"io"
	"sync"
	"time"
)

const updateTimeout = time.Second * 5

type cacheEntry struct {
	Value   []byte
	Created time.Time
}

var cache sync.Map

func callAndCache(r *request, f func(io.Writer, *request), key string, now time.Time) []byte {
	var buf bytes.Buffer

	clone := *r
	clone.Context = context.Background()

	f(&buf, &clone)

	resp := buf.Bytes()
	cache.Store(key, cacheEntry{append(resp, []byte(fmt.Sprintf("\n\n(Cached response generated on %s)", now.Format(time.UnixDate)))...), now})
	return resp
}

func withCache(f func(io.Writer, *request), d time.Duration) func(io.Writer, *request) {
	return func(w io.Writer, r *request) {
		key := r.URL.String()
		now := time.Now()

		entry, cached := cache.Load(key)
		if !cached {
			r.Log.WithField("key", key).Info("Generating first response")
			w.Write(callAndCache(r, f, key, now))
			return
		}

		if entry.(cacheEntry).Created.After(now.Add(-d)) {
			r.Log.WithField("key", key).Info("Sending cached response")
			w.Write(entry.(cacheEntry).Value)
			return
		}

		update := make(chan []byte, 1)
		timer := time.NewTimer(updateTimeout)
		defer timer.Stop()

		r.WaitGroup.Add(1)
		go func() {
			r.Log.WithField("key", key).Info("Generating new response")
			update <- callAndCache(r, f, key, now)
			r.WaitGroup.Done()
		}()

		select {
		case resp := <-update:
			w.Write(resp)

		case <-timer.C:
			r.Log.WithField("key", key).Warn("Timeout, sending old cached response")
			w.Write(entry.(cacheEntry).Value)
		}
	}
}
