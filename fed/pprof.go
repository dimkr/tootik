//go:build !no_pprof

/*
Copyright 2025 Dima Krasner

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

package fed

import (
	"log/slog"
	"math/rand/v2"
	"net/http"
	"net/http/pprof"
	"strings"
)

func withoutPrefix(h func(http.ResponseWriter, *http.Request), prefix string) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		h(w, r)
	}
}

func generatePrefix() string {
	b := make([]byte, 33)

	b[0] = '/'
	for i := 1; i < 33; i++ {
		b[i] = 'a' + byte(rand.IntN('z'-'a'))
	}

	return string(b)
}

func (l *Listener) withPprof(inner http.Handler) (http.Handler, error) {
	mux := http.NewServeMux()
	mux.Handle("/", inner)

	prefix := generatePrefix()
	slog.Info("Enabling pprof", "url", "https://"+l.Domain+prefix+"/debug/pprof")

	mux.HandleFunc("GET "+prefix+"/debug/pprof/", withoutPrefix(pprof.Index, prefix))
	mux.HandleFunc("GET "+prefix+"/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET "+prefix+"/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET "+prefix+"/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET "+prefix+"/debug/pprof/trace", pprof.Trace)

	return mux, nil
}
