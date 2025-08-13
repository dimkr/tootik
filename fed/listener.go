/*
Copyright 2023 - 2025 Dima Krasner

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
	"log/slog"
	"math"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/httpsig"
	"github.com/fsnotify/fsnotify"
)

type Listener struct {
	Domain    string
	Closed    bool
	Config    *cfg.Config
	DB        *sql.DB
	Resolver  *Resolver
	ActorKeys [2]httpsig.Key
	Addr      string
	Cert      string
	Key       string
	Plain     bool
}

const certReloadDelay = time.Second * 5

// NewHandler returns a [http.Handler] that handles ActivityPub requests.
func (l *Listener) NewHandler() (http.Handler, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /robots.txt", robots)
	mux.HandleFunc("GET /.well-known/webfinger", l.handleWebFinger)
	mux.HandleFunc("GET /.well-known/apgateway/{resource...}", l.handleAPGatewayGet)
	mux.HandleFunc("POST /.well-known/apgateway/{resource...}", l.handleAPGatewayPost)
	mux.HandleFunc("GET /user/{username}", l.handleUser)
	mux.HandleFunc("GET /icon/{username}", l.handleIcon)
	mux.HandleFunc("POST /inbox/{username}", l.handleInbox)
	mux.HandleFunc("GET /outbox/{username}", l.handleOutbox)
	mux.HandleFunc("GET /post/{hash}", l.handlePost)
	mux.HandleFunc("GET /create/{hash}", l.handleCreate)
	mux.HandleFunc("GET /update/{hash}", l.handleUpdate)
	mux.HandleFunc("GET /followers_synchronization/{username}", l.handleFollowers)
	mux.HandleFunc("GET /{$}", l.handleIndex)

	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("Received request to non-existing path", "path", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})

	if err := addNodeInfo(mux, l.Domain, l.Closed, l.Config, l.DB); err != nil {
		return nil, err
	}

	addHostMeta(mux, l.Domain)

	return mux, nil
}

// ListenAndServe handles HTTP requests from other servers.
func (l *Listener) ListenAndServe(ctx context.Context) error {
	mux, err := l.NewHandler()
	if err != nil {
		return err
	}

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
			Addr:    l.Addr,
			Handler: http.TimeoutHandler(mux, time.Second*30, ""),
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

			// shut down gracefully only on reload
			if ctx.Err() == nil {
				slog.Info("Shutting down server")
				server.Shutdown(ctx)
			}

			server.Close()
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
					return

				case event, ok := <-w.Events:
					if !ok {
						continue
					}

					if (event.Has(fsnotify.Write) || event.Has(fsnotify.Create)) && (event.Name == certAbsPath || event.Name == keyAbsPath) {
						slog.Info("Stopping server: file has changed", "name", event.Name)
						timer.Reset(certReloadDelay)
					}

				case <-timer.C:
					server.Shutdown(context.Background())
					return

				case <-w.Errors:
				}
			}
		}()

		slog.Info("Starting server")
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
