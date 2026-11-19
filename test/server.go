/*
Copyright 2023 - 2026 Dima Krasner

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
	"database/sql"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/test/deps"
)

const domain = "localhost.localdomain:8443"

type server struct {
	t            *testing.T
	cfg          *cfg.Config
	dbw          *sql.DB
	resolver     *fed.Resolver
	handler      front.Handler
	inbox        *inbox.Inbox
	queue        *inbox.Queue
	Alice        *ap.Actor
	Bob          *ap.Actor
	Carol        *ap.Actor
	AppActor     *ap.Actor
	AppActorKeys [3]httpsig.Key
}

func newTestServer(t *testing.T) *server {
	t.Parallel()

	db := deps.NewDatabase(t)

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.DisableIntegrityProofs = true

	alice, _, err := user.Create(t.Context(), domain, db.DBW, &cfg, "alice", nil)
	if err != nil {
		t.Fatalf("Failed to create alice: %v", err)
	}

	bob, _, err := user.Create(t.Context(), domain, db.DBW, &cfg, "bob", nil)
	if err != nil {
		t.Fatalf("Failed to create bob: %v", err)
	}

	carol, _, err := user.Create(t.Context(), domain, db.DBW, &cfg, "carol", nil)
	if err != nil {
		t.Fatalf("Failed to create carol: %v", err)
	}

	appActor, appActorKeys, err := user.CreateApplicationActor(t.Context(), domain, db.DBW, &cfg)
	if err != nil {
		t.Fatalf("Failed to create the application actor: %v", err)
	}

	localInbox := &inbox.Inbox{
		Domain: domain,
		Config: &cfg,
		DBW:    db.DBW,
	}

	resolver := fed.NewResolver(nil, domain, &cfg, &http.Client{}, db.DBW)

	queue := &inbox.Queue{
		Domain:   domain,
		Config:   &cfg,
		DBW:      db.DBW,
		Inbox:    localInbox,
		Resolver: resolver,
		Keys:     appActorKeys,
	}

	handler, err := front.NewHandler(domain, &cfg, resolver, db.DB, db.DBW, localInbox)
	if err != nil {
		t.Fatalf("Failed to create a handler: %v", err)
	}

	return &server{
		t:            t,
		cfg:          &cfg,
		dbw:          db.DBW,
		resolver:     resolver,
		handler:      handler,
		inbox:        localInbox,
		queue:        queue,
		Alice:        alice,
		Bob:          bob,
		Carol:        carol,
		AppActor:     appActor,
		AppActorKeys: appActorKeys,
	}
}

func (s *server) handle(u *url.URL, user *ap.Actor, body io.Reader) string {
	r := front.Request{
		Context: s.t.Context(),
		URL:     u,
		Log:     slog.Default(),
		Body:    body,
		User:    user,
	}

	var buf bytes.Buffer
	w := gmi.Wrap(&buf)
	s.handler.Handle(&r, w)
	w.Flush()

	return buf.String()
}

func (s *server) Handle(request string, user *ap.Actor) string {
	u, err := url.Parse(request)
	if err != nil {
		s.t.Fatalf("Failed to parse %s: %v", request, err)
	}

	return s.handle(u, user, nil)
}

func (s *server) Upload(request string, user *ap.Actor, body []byte) string {
	u, err := url.Parse(request)
	if err != nil {
		s.t.Fatalf("Failed to parse %s: %v", request, err)
	}
	u.Scheme = "titan"

	return s.handle(u, user, bytes.NewBuffer(body))
}
