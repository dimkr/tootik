/*
Copyright 2024 Dima Krasner

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
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/migrations"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
)

type instance struct {
	Log           *slog.Logger
	Config        *cfg.Config
	DB            *sql.DB
	Resolver      *fed.Resolver
	DeliveryQueue *fed.Queue
	IncomingQueue *inbox.Queue
	Frontend      front.Handler
	Backend       http.Handler
	Alice         *ap.Actor
	AliceKey      httpsig.Key
	Bob           *ap.Actor
	BobKey        httpsig.Key
	ActorKey      httpsig.Key
	t             *testing.T
}

type fediverseClient struct {
	Fediverse *fediverse
}

type fediverse struct {
	Instances map[string]*instance
	Client    fediverseClient
	t         *testing.T
}

type testResponseWriter struct {
	StatusCode int
	Headers    http.Header
	Body       bytes.Buffer
	t          *testing.T
}

func (i *instance) Shutdown() {
	i.DB.Close()
}

func (i *instance) UpdateFeed() {
	(inbox.FeedUpdater{
		Domain: i.Frontend.Domain,
		Config: i.Config,
		DB:     i.DB,
	}).Run(context.Background())
}

func NewFediverse(t *testing.T) fediverse {
	fediverse := fediverse{
		Instances: map[string]*instance{},
		t:         t,
	}

	fediverse.Client = fediverseClient{
		Fediverse: &fediverse,
	}

	return fediverse
}

func (f fediverse) Shutdown() {
	for _, instance := range f.Instances {
		instance.Shutdown()
	}
}

func (c *fediverseClient) Do(r *http.Request) (*http.Response, error) {
	w := &testResponseWriter{
		Headers: http.Header{},
		t:       c.Fediverse.t,
	}

	i, ok := c.Fediverse.Instances[r.URL.Host]
	if !ok {
		return nil, fmt.Errorf("Unknown instance: %s", r.URL.Host)
	}

	i.Backend.ServeHTTP(w, r)

	return &http.Response{
		StatusCode: w.StatusCode,
		Header:     w.Headers,
		Body:       io.NopCloser(&w.Body),
	}, nil
}

func (w *testResponseWriter) Header() http.Header {
	return w.Headers
}

func (w *testResponseWriter) Write(p []byte) (int, error) {
	if w.StatusCode == 0 {
		w.StatusCode = http.StatusOK
	}
	return w.Body.Write(p)
}

func (w *testResponseWriter) WriteHeader(statusCode int) {
	if w.StatusCode == 0 {
		w.StatusCode = statusCode
	} else {
		w.t.Fatalf("Status code is already set to %d", w.StatusCode)
	}
}

func (i *instance) HandleFrontendRequest(request string, user *ap.Actor, key httpsig.Key) string {
	u, err := url.Parse(request)
	if err != nil {
		panic(err)
		i.t.FailNow()
	}

	var buf bytes.Buffer
	var wg sync.WaitGroup
	w := gmi.Wrap(&buf)
	i.Frontend.Handle(context.Background(), i.Log, nil, w, u, user, key, i.DB, i.Resolver, &wg)
	w.Flush()

	return buf.String()
}

func (f fediverse) AddInstance(domain string) *instance {
	log := slog.Default().With("instance", domain)

	db, err := sql.Open("sqlite3", filepath.Join(f.t.TempDir(), domain+".sqlite3")+"?_journal_mode=WAL")
	if err != nil {
		panic(err)
		f.t.FailNow()
	}

	if err := migrations.Run(context.Background(), log, domain, db); err != nil {
		panic(err)
		f.t.FailNow()
	}

	_, nobodyKey, err := user.CreateNobody(context.Background(), domain, db)
	if err != nil {
		panic(err)
		f.t.FailNow()
	}

	alice, aliceKey, err := user.Create(context.Background(), domain, db, "alice", ap.Person, nil)
	if err != nil {
		panic(err)
		f.t.FailNow()
	}

	bob, bobKey, err := user.Create(context.Background(), domain, db, "bob", ap.Person, nil)
	if err != nil {
		panic(err)
		f.t.FailNow()
	}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0
	cfg.EditThrottleUnit = 0

	resolver := fed.NewResolver(nil, domain, &cfg, &f.Client)

	l := fed.Listener{
		Domain:   domain,
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
		ActorKey: nobodyKey,
		Log:      log,
	}

	q := fed.Queue{
		Domain:   domain,
		Config:   &cfg,
		Log:      log,
		DB:       db,
		Resolver: resolver,
	}

	iq := inbox.Queue{
		Domain:   domain,
		Config:   &cfg,
		Log:      log,
		DB:       db,
		Resolver: resolver,
		Key:      nobodyKey,
	}

	frontend, err := front.NewHandler(domain, false, &cfg)
	if err != nil {
		panic(err)
		f.t.FailNow()
	}

	h, err := l.GetHandler()
	if err != nil {
		panic(err)
		f.t.FailNow()
	}

	i := &instance{
		Log:           log,
		Config:        &cfg,
		DB:            db,
		Resolver:      resolver,
		DeliveryQueue: &q,
		IncomingQueue: &iq,
		Frontend:      frontend,
		Backend:       h,
		Alice:         alice,
		AliceKey:      aliceKey,
		Bob:           bob,
		BobKey:        bobKey,
		ActorKey:      nobodyKey,
		t:             f.t,
	}

	f.Instances[domain] = i

	return i
}

func TestFederation_CreateUpdateDelete(t *testing.T) {
	fediverse := NewFediverse(t)
	defer fediverse.Shutdown()

	foo := fediverse.AddInstance("foo.localdomain")
	bar := fediverse.AddInstance("bar.localdomain")

	if resp := bar.HandleFrontendRequest("/users/resolve?alice%40foo.localdomain", bar.Bob, bar.BobKey); resp != "30 /users/outbox/foo.localdomain/user/alice\r\n" {
		t.Fatalf("Failed to resolve alice@foo.localdomain: %s", resp)
	}

	if resp := bar.HandleFrontendRequest("/users/follow/foo.localdomain/user/alice", bar.Bob, bar.BobKey); resp != "30 /users/outbox/foo.localdomain/user/alice\r\n" {
		t.Fatalf("Failed to follow alice@foo.localdomain: %s", resp)
	}

	// send the Follow activity
	if err := bar.DeliveryQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to send Follow: %v", err)
	}

	// process the Follow activity and queue an Accept activity
	if _, err := foo.IncomingQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to queue Accept: %v", err)
	}

	// send the Accept activity
	if err := foo.DeliveryQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to send Accept: %v", err)
	}

	// process the Accept activity
	if _, err := bar.IncomingQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to process Accept: %v", err)
	}

	resp := foo.HandleFrontendRequest("/users/say?Hello%20world", foo.Alice, foo.AliceKey)
	if !strings.HasPrefix(resp, "30 /users/view/foo.localdomain/post/") {
		t.Fatalf("Failed to post as alice@foo.localdomain: %s", resp)
	}
	postID := resp[15 : len(resp)-2]

	// send the Create activity
	if err := foo.DeliveryQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to send Create: %v", err)
	}

	// process the Create activity
	if _, err := bar.IncomingQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to process Create: %v", err)
	}

	bar.UpdateFeed()
	if resp := bar.HandleFrontendRequest("/users", bar.Bob, bar.BobKey); !slices.Contains(strings.Split(resp, "\n"), "> Hello world") {
		t.Fatalf("Failed to view post: %s", resp)
	}

	if resp := bar.HandleFrontendRequest("/users/view/"+postID, bar.Bob, bar.BobKey); !slices.Contains(strings.Split(resp, "\n"), "> Hello world") {
		t.Fatalf("Failed to view post: %s", resp)
	}

	if resp := bar.HandleFrontendRequest("/users/fts?hello", bar.Bob, bar.BobKey); !slices.Contains(strings.Split(resp, "\n"), "> Hello world") {
		t.Fatalf("Failed to view post: %s", resp)
	}

	if resp := foo.HandleFrontendRequest(fmt.Sprintf("/users/edit/%s?Hello", postID), foo.Alice, foo.AliceKey); resp != fmt.Sprintf("30 /users/view/%s\r\n", postID) {
		t.Fatalf("Failed to edit post: %s", resp)
	}

	// send the Update activity
	if err := foo.DeliveryQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to send Update: %v", err)
	}

	// process the Update activity
	if _, err := bar.IncomingQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to process Update: %v", err)
	}

	if resp := bar.HandleFrontendRequest("/users", bar.Bob, bar.BobKey); !slices.Contains(strings.Split(resp, "\n"), "> Hello") {
		t.Fatalf("Original post is still visible: %s", resp)
	}

	if resp := bar.HandleFrontendRequest("/users/view/"+postID, bar.Bob, bar.BobKey); !slices.Contains(strings.Split(resp, "\n"), "> Hello") {
		t.Fatalf("Original post is still visible: %s", resp)
	}

	if resp := bar.HandleFrontendRequest("/users/fts?hello", bar.Bob, bar.BobKey); !slices.Contains(strings.Split(resp, "\n"), "> Hello") {
		t.Fatalf("Original post is still visible: %s", resp)
	}

	if resp := foo.HandleFrontendRequest("/users/delete/"+postID, foo.Alice, foo.AliceKey); resp != "30 /users/outbox/foo.localdomain/user/alice\r\n" {
		t.Fatalf("Failed to delete post: %s", resp)
	}

	// send the Undo activity
	if err := foo.DeliveryQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to send Undo: %v", err)
	}

	// process the Undo activity
	if _, err := bar.IncomingQueue.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to process Undo: %v", err)
	}

	if resp := bar.HandleFrontendRequest("/users", bar.Bob, bar.BobKey); slices.Contains(strings.Split(resp, "\n"), "> Hello") {
		t.Fatalf("Original post is still visible: %s", resp)
	}

	if resp := bar.HandleFrontendRequest("/users/view/"+postID, bar.Bob, bar.BobKey); resp != "40 Post not found\r\n" {
		t.Fatalf("Deleted post is still visible: %s", resp)
	}

	if resp := bar.HandleFrontendRequest("/users/fts?hello", bar.Bob, bar.BobKey); slices.Contains(strings.Split(resp, "\n"), "> Hello") {
		t.Fatalf("Deleted post is still visible: %s", resp)
	}
}
