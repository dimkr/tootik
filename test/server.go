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

package test

import (
	"bytes"
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"net/url"
	"os"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/migrations"
	_ "github.com/mattn/go-sqlite3"
)

const domain = "localhost.localdomain:8443"

type server struct {
	cfg       *cfg.Config
	db        *sql.DB
	dbPath    string
	handler   front.Handler
	Alice     *ap.Actor
	Bob       *ap.Actor
	Carol     *ap.Actor
	NobodyKey httpsig.Key
}

func (s *server) Shutdown() {
	s.db.Close()
	os.Remove(s.dbPath)
}

func newTestServer() *server {
	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	if err != nil {
		panic(err)
	}
	f.Close()

	path := f.Name()

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	if err != nil {
		panic(err)
	}

	var cfg cfg.Config
	cfg.FillDefaults()

	if err := migrations.Run(context.Background(), domain, db); err != nil {
		panic(err)
	}

	alice, _, err := user.Create(context.Background(), domain, db, "alice", ap.Person, nil, user.GenerateRSAKey)
	if err != nil {
		panic(err)
	}

	bob, _, err := user.Create(context.Background(), domain, db, "bob", ap.Person, nil, user.GenerateRSAKey)
	if err != nil {
		panic(err)
	}

	carol, _, err := user.Create(context.Background(), domain, db, "carol", ap.Person, nil, user.GenerateRSAKey)
	if err != nil {
		panic(err)
	}

	_, nobodyKey, err := user.CreateNobody(context.Background(), domain, db, user.GenerateRSAKey)
	if err != nil {
		panic(err)
	}

	handler, err := front.NewHandler(domain, false, &cfg, fed.NewResolver(nil, domain, &cfg, &http.Client{}, db), db, user.GenerateRSAKey)
	if err != nil {
		panic(err)
	}

	return &server{
		cfg:       &cfg,
		dbPath:    path,
		db:        db,
		handler:   handler,
		Alice:     alice,
		Bob:       bob,
		Carol:     carol,
		NobodyKey: nobodyKey,
	}
}

func (s *server) Handle(request string, user *ap.Actor) string {
	u, err := url.Parse(request)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	w := gmi.Wrap(&buf)
	s.handler.Handle(
		&front.Request{
			Context: context.Background(),
			URL:     u,
			Log:     slog.Default(),
			User:    user,
		},
		w,
	)
	w.Flush()

	return buf.String()
}

func (s *server) Upload(request string, user *ap.Actor, body []byte) string {
	u, err := url.Parse(request)
	if err != nil {
		panic(err)
	}
	u.Scheme = "titan"

	var buf bytes.Buffer
	w := gmi.Wrap(&buf)
	s.handler.Handle(
		&front.Request{
			Context: context.Background(),
			URL:     u,
			Log:     slog.Default(),
			Body:    bytes.NewBuffer(body),
			User:    user,
		},
		w,
	)
	w.Flush()

	return buf.String()
}
