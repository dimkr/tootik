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
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"net"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.URL)
}

func robots(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("User-agent: *\n"))
	w.Write([]byte("Disallow: /\n"))
}

func ListenAndServe(ctx context.Context, db *sql.DB, addr, cert, key string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler)
	mux.HandleFunc("/robots.txt", robots)
	mux.HandleFunc("/.well-known/webfinger",
		func(w http.ResponseWriter, r *http.Request) {
			webFingerHandler(w, r, db)
		})
	mux.HandleFunc("/user/",
		func(w http.ResponseWriter, r *http.Request) {
			userHandler(w, r, db)
		})
	mux.HandleFunc("/inbox/",
		func(w http.ResponseWriter, r *http.Request) {
			inboxHandler(w, r, db)
		})
	mux.HandleFunc("/outbox/",
		func(w http.ResponseWriter, r *http.Request) {
			outboxHandler(w, r, db)
		})

	server := http.Server{
		Addr:    addr,
		Handler: mux,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
	}

	go func() {
		<-ctx.Done()
		server.Shutdown(context.Background())
	}()

	return server.ListenAndServeTLS(cert, key)
}
