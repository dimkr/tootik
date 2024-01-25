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

package fed

import (
	"context"
	"crypto/tls"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/fsnotify/fsnotify"
	"log/slog"
	"math"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"
)

type Listener struct {
	Domain   string
	LogLevel slog.Level
	Config   *cfg.Config
	DB       *sql.DB
	Resolver *Resolver
	Actor    *ap.Actor
	Log      *slog.Logger
	Addr     string
	Cert     string
	Key      string
	Plain    bool
}

const certReloadDelay = time.Second * 5

func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("User-agent: *\n"))
	w.Write([]byte("Disallow: /\n"))
}

// ListenAndServe handles HTTP requests from other servers.
func (l *Listener) ListenAndServe(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", robots)
	mux.HandleFunc("/.well-known/webfinger", func(w http.ResponseWriter, r *http.Request) {
		handler := webFingerHandler{Listener: l, Log: l.Log.With("query", r.URL.RawQuery)}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/user/", func(w http.ResponseWriter, r *http.Request) {
		handler := userHandler{Listener: l, Log: l.Log.With(slog.String("path", r.URL.Path))}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/icon/", func(w http.ResponseWriter, r *http.Request) {
		handler := iconHandler{Listener: l, Log: l.Log.With(slog.String("path", r.URL.Path))}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/inbox/", func(w http.ResponseWriter, r *http.Request) {
		handler := inboxHandler{Listener: l, Log: l.Log.With(slog.String("path", r.URL.Path))}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/outbox/", func(w http.ResponseWriter, r *http.Request) {
		handler := outboxHandler{Listener: l, Log: l.Log.With(slog.String("path", r.URL.Path))}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/post/", func(w http.ResponseWriter, r *http.Request) {
		handler := postHandler{Listener: l, Log: l.Log.With(slog.String("path", r.URL.Path))}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Location", fmt.Sprintf("gemini://%s", l.Domain))
			w.WriteHeader(http.StatusMovedPermanently)
		} else {
			l.Log.Debug("Received request to non-existing path", "path", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})

	if err := addNodeInfo(mux, l.Domain); err != nil {
		return err
	}

	addHostMeta(mux, l.Domain)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	certDir := filepath.Dir(l.Cert)
	certAbsPath := filepath.Join(certDir, filepath.Base(l.Cert))

	keyDir := filepath.Dir(l.Key)
	keyAbsPath := filepath.Join(keyDir, filepath.Base(l.Key))

	if !l.Plain {
		if err := w.Add(certDir); err != nil {
			return err
		}

		if keyDir != certDir {
			if err := w.Add(keyDir); err != nil {
				return err
			}
		}
	}

	for ctx.Err() == nil {
		var wg sync.WaitGroup
		serverCtx, stopServer := context.WithCancel(ctx)

		server := http.Server{
			Addr:     l.Addr,
			Handler:  mux,
			ErrorLog: slog.NewLogLogger(l.Log.Handler(), l.LogLevel),
			BaseContext: func(net.Listener) context.Context {
				return serverCtx
			},
			ReadTimeout: time.Second * 30,
			TLSConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
		}

		wg.Add(1)
		go func() {
			<-serverCtx.Done()
			server.Shutdown(context.Background())
			wg.Done()
		}()

		timer := time.NewTimer(math.MaxInt64)
		timer.Stop()

		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				select {
				case <-serverCtx.Done():
					server.Shutdown(context.Background())
					return

				case event, ok := <-w.Events:
					if !ok {
						continue
					}

					if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) && (event.Name == certAbsPath || event.Name == keyAbsPath) {
						l.Log.Info("Stopping server: file has changed", "name", event.Name)
						timer.Reset(certReloadDelay)
					}

				case <-timer.C:
					server.Shutdown(context.Background())
					return

				case <-w.Errors:
				}
			}
		}()

		l.Log.Info("Starting server")
		var err error
		if l.Plain {
			err = server.ListenAndServe()
		} else {
			err = server.ListenAndServeTLS(l.Cert, l.Key)
		}

		stopServer()
		wg.Wait()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	return nil
}
