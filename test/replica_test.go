/*
Copyright 2026 Dima Krasner

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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/test/deps"
	"github.com/stretchr/testify/assert"
)

// copyStale copies a post and its author from the primary to a database that never receives the server's writes, with
// different content, so reads from it are distinguishable from reads from the primary.
func copyStale(t *testing.T, server *server, stale *deps.Database, id, content string) {
	t.Helper()

	var note ap.Object
	var slug, author string
	var public bool
	if err := server.dbw.QueryRowContext(t.Context(), `select slug, author, object, public from notes where id = $1`, id).Scan(&slug, &author, &note, &public); err != nil {
		t.Fatalf("Failed to fetch %s: %v", id, err)
	}

	var actorSlug string
	var actor ap.Actor
	if err := server.dbw.QueryRowContext(t.Context(), `select slug, actor from persons where id = $1`, author).Scan(&actorSlug, &actor); err != nil {
		t.Fatalf("Failed to fetch %s: %v", author, err)
	}

	if _, err := stale.DBW.ExecContext(t.Context(), `insert into persons(slug, id, actor) values($1, $2, $3) on conflict do nothing`, actorSlug, author, &actor); err != nil {
		t.Fatalf("Failed to copy %s: %v", author, err)
	}

	note.Content = content
	if _, err := stale.DBW.ExecContext(t.Context(), `insert into notes(slug, id, author, object, public) values($1, $2, $3, $4, $5)`, slug, id, author, &note, public); err != nil {
		t.Fatalf("Failed to copy %s: %v", id, err)
	}
}

func TestReplica_ReadsOwnPostsFromPrimary(t *testing.T) {
	server := newTestServer(t)

	assert := assert.New(t)

	stale := deps.NewDatabase(t)

	handler, err := front.NewHandler(domain, server.cfg, server.resolver, stale.DBW, server.dbw, server.inbox)
	if err != nil {
		t.Fatalf("Failed to create a handler: %v", err)
	}
	server.handler = handler

	backend, err := (&fed.Listener{
		Domain:       domain,
		Config:       server.cfg,
		DB:           stale.DBW,
		DBW:          server.dbw,
		Resolver:     server.resolver,
		AppActor:     server.AppActor,
		AppActorKeys: server.AppActorKeys,
	}).NewHandler()
	if err != nil {
		t.Fatalf("Failed to create a federation handler: %v", err)
	}

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)
	postPath := say[15 : len(say)-2]
	copyStale(t, server, stale, "https://"+postPath, "Stale hello")

	other := server.Handle("/users/say?Hello%20from%20Carol", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, other)
	otherPath := other[15 : len(other)-2]

	view := server.Handle("/users/view/"+postPath, server.Alice)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Stale hello")

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Alice)
	assert.Contains(outbox, "Hello world")
	assert.NotContains(outbox, "Stale hello")

	view = server.Handle("/users/view/"+postPath, server.Bob)
	assert.Contains(view, "Stale hello")
	assert.NotContains(view, "Hello world")

	outbox = server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Stale hello")
	assert.NotContains(outbox, "Hello world")

	local := server.Handle("/users/local", server.Bob)
	assert.Contains(local, "Stale hello")
	assert.NotContains(local, "Hello from Carol")

	for path, status := range map[string]int{
		postPath:  http.StatusOK,
		otherPath: http.StatusNotFound,
	} {
		r := httptest.NewRequest(http.MethodGet, "https://"+path, nil)
		r.Header.Set("Accept", "application/activity+json")
		w := httptest.NewRecorder()
		backend.ServeHTTP(w, r)
		assert.Equal(status, w.Code, path)
	}

	bookmark := server.Handle("/users/bookmark/"+postPath, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", postPath), bookmark)

	view = server.Handle("/users/view/"+postPath, server.Bob)
	assert.Contains(strings.Split(view, "\n"), fmt.Sprintf("=> /users/unbookmark/%s 🔖 Unbookmark", postPath))

	bookmarks := server.Handle("/users/bookmarks", server.Bob)
	assert.Contains(bookmarks, "> Hello world")
}
