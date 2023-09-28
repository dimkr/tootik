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
	"net"
	"net/http"
	"sync"
	"time"
)

func robots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("User-agent: *\n"))
	w.Write([]byte("Disallow: /\n"))
}

func root(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", fmt.Sprintf("gemini://%s", cfg.Domain))
	w.WriteHeader(http.StatusMovedPermanently)
}

func ListenAndServe(ctx context.Context, db *sql.DB, resolver *Resolver, actor *ap.Actor, log *slog.Logger, addr, cert, key string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", robots)
	mux.HandleFunc("/.well-known/webfinger", func(w http.ResponseWriter, r *http.Request) {
		handler := webFingerHandler{log.With("query", r.URL.RawQuery), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/user/", func(w http.ResponseWriter, r *http.Request) {
		handler := userHandler{log.With(slog.String("path", r.URL.Path)), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/icon/", func(w http.ResponseWriter, r *http.Request) {
		handler := iconHandler{log.With(slog.String("path", r.URL.Path)), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/inbox/", func(w http.ResponseWriter, r *http.Request) {
		handler := inboxHandler{log.With(slog.String("path", r.URL.Path)), db, resolver, actor}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/outbox/", func(w http.ResponseWriter, r *http.Request) {
		handler := outboxHandler{log.With(slog.String("path", r.URL.Path)), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/post/", func(w http.ResponseWriter, r *http.Request) {
		handler := postHandler{log.With(slog.String("path", r.URL.Path)), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/", root)

	if err := addNodeInfo(mux); err != nil {
		return err
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer w.Close()

	if err := w.Add(cert); err != nil {
		return err
	}
	if err := w.Add(key); err != nil {
		return err
	}

	for ctx.Err() == nil {
		var wg sync.WaitGroup
		serverCtx, stopServer := context.WithCancel(ctx)

		server := http.Server{
			Addr:     addr,
			Handler:  mux,
			ErrorLog: slog.NewLogLogger(log.Handler(), slog.Level(cfg.LogLevel)),
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

					if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Rename) {
						continue
					}

					log.Info("Stopping HTTPS server: file has changed", "name", event.Name)
					server.Shutdown(context.Background())
					return

				case <-w.Errors:
				}
			}
		}()

		log.Info("Starting HTTPS server")
		err := server.ListenAndServeTLS(cert, key)

		stopServer()
		wg.Wait()

		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}

	return nil
}
