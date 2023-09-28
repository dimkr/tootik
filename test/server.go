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

package test

import (
	"sync"

	"bytes"
	"context"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/user"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
	"net/url"
	"os"
)

type server struct {
	db      *sql.DB
	dbPath  string
	handler front.Handler
	Alice   *ap.Actor
	Bob     *ap.Actor
	Carol   *ap.Actor
	Nobody  *ap.Actor
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

	if err := migrations.Run(context.Background(), slog.Default(), db); err != nil {
		panic(err)
	}

	alice, err := user.Create(context.Background(), db, fmt.Sprintf("https://%s/user/alice", cfg.Domain), "alice", "a")
	if err != nil {
		panic(err)
	}

	bob, err := user.Create(context.Background(), db, fmt.Sprintf("https://%s/user/bob", cfg.Domain), "bob", "b")
	if err != nil {
		panic(err)
	}

	carol, err := user.Create(context.Background(), db, fmt.Sprintf("https://%s/user/carol", cfg.Domain), "carol", "c")
	if err != nil {
		panic(err)
	}

	nobody, err := user.CreateNobody(context.Background(), db)
	if err != nil {
		panic(err)
	}

	return &server{
		dbPath:  path,
		db:      db,
		handler: front.NewHandler(),
		Alice:   alice,
		Bob:     bob,
		Carol:   carol,
		Nobody:  nobody,
	}
}

func (s *server) Handle(request string, user *ap.Actor) string {
	u, err := url.Parse(request)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer
	var wg sync.WaitGroup
	s.handler.Handle(context.Background(), slog.Default(), gmi.Wrap(&buf), u, user, s.db, fed.NewResolver(nil), &wg)

	return string(buf.Bytes())
}
