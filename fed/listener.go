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
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/logger"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
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

func ListenAndServe(ctx context.Context, db *sql.DB, addr, cert, key string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/robots.txt", robots)
	mux.HandleFunc("/.well-known/webfinger", func(w http.ResponseWriter, r *http.Request) {
		handler := webFingerHandler{logger.New(log.Fields{"query": r.URL.RawQuery}), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/user/", func(w http.ResponseWriter, r *http.Request) {
		handler := userHandler{logger.New(log.Fields{"path": r.URL.Path}), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/icon/", func(w http.ResponseWriter, r *http.Request) {
		handler := iconHandler{logger.New(log.Fields{"path": r.URL.Path}), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/inbox/", func(w http.ResponseWriter, r *http.Request) {
		handler := inboxHandler{logger.New(log.Fields{"path": r.URL.Path}), db}
		handler.Handle(w, r)
	})
	mux.HandleFunc("/", root)

	server := http.Server{
		Addr:    addr,
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		ReadTimeout: time.Second * 30,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	if err := server.ListenAndServeTLS(cert, key); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}
