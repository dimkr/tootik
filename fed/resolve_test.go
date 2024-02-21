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

package fed

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/dimkr/tootik/migrations"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"io"
	"log/slog"
	"net/http"
	"os"
	"testing"
)

type testResponse struct {
	Response *http.Response
	Error    error
}

type testClient map[string]testResponse

func (c testClient) Do(r *http.Request) (*http.Response, error) {
	url := r.URL.String()
	resp, ok := c[url]
	if !ok {
		panic("No response for " + url)
	}
	delete(c, url)
	return resp.Response, resp.Error
}

func TestResolve_LocalActor(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, nobody.ID, 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal(nobody.ID, actor.ID)
	assert.Equal(nobody.Inbox, actor.Inbox)
}

func TestResolve_LocalActorDoesNotExist(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://localhost.localdomain/user/doesnotexist", 0)
	assert.True(errors.Is(err, ErrNoLocalActor))
}

func TestResolve_FederatedActorInvalidURL(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan%zz", 0)
	assert.Error(err)
}

func TestResolve_FederatedActorInvalidScheme(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "http://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrInvalidScheme))
}

func TestResolve_FederatedActorEmptyName(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/@", 0)
	assert.Error(err)
}

func TestResolve_FederatedActorFirstTime(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorFirstTimeThroughMention(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/@dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorFirstTimeOffline(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", ap.Offline)
	assert.True(errors.Is(err, ErrActorNotCached))
	assert.Empty(client)
}

func TestResolve_FederatedActorFirstTimeCancelled(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	for i := range resolver.locks {
		assert.NoError(resolver.locks[i].Acquire(context.Background(), 1))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = resolver.ResolveID(ctx, slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, context.Canceled))
	assert.Empty(client)
}

func TestResolve_FederatedActorFirstTimeInvalidWebFingerLink(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "123",
								"rel": "abc",
								"type": "def"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorFirstTimeActorIDMismatch(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/erin",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.Error(err)
	assert.Empty(client)
}

func TestResolve_FederatedActorCached(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorCachedInvalidActorHost(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://169.254.0.1/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://169.254.0.1/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrInvalidHost))
	assert.Empty(client)
}

func TestResolve_FederatedActorCachedActorHostWithPort(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0:443/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0:443/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrInvalidHost))
	assert.Empty(client)
}

func TestResolve_FederatedActorCachedActorHostSubdomain(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://tootik.0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://tootik.0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://tootik.0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://tootik.0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://tootik.0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://tootik.0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://tootik.0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorCachedActorHostSubdomainFetchedRecently(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://tootik.0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://tootik.0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://tootik.0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://tootik.0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	client = testClient{}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://tootik.0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorCachedActorIDChanged(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/erin"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/erin",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/erin",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:erin@0.0.0.0"
					}`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorCachedButBlocked(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	blockList.domains = map[string]struct{}{
		"0.0.0.0": struct{}{},
	}

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrBlockedDomain))
}

func TestResolve_FederatedActorOldCache(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan123",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheWasNew(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan123",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						},
						"published": "2018-08-18T00:00:00Z"
					}`))),
			},
		},
	}

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheStillNew(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client)
}

func TestResolve_FederatedActorOldCacheWasOld(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						},
						"published": "2018-08-18T00:00:00Z"
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						},
						"published": "2088-08-18T00:00:00Z"
					}`))),
			},
		},
	}

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client)
}

func TestResolve_FederatedActorOldCacheWasNewNowUnknown(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						},
						"published": "2088-08-18T00:00:00Z"
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client)
}

func TestResolve_FederatedActorOldCacheFetchedRecently(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*5 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheButOffline(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", ap.Offline)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheInvalidID(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "http://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "http://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrInvalidID))
	assert.Empty(client)
}

func TestResolve_FederatedActorOldCacheInvalidWebFingerResponse(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`abc`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheBigWebFingerResponse(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
	}

	cfg.MaxRequestBodySize = 1

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheInvalidActor(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`abc`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheBigActor(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan123",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	cfg.MaxRequestBodySize = 438

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorNoProfileLink(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "abc"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "def"
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheWebFingerError(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Error: errors.New("a"),
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheActorError(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Error: errors.New("a"),
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheActorDeleted(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/user/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/user/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	tx, err := db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://0.0.0.0/note/1",
				Type:         ap.Note,
				AttributedTo: "https://0.0.0.0/user/dan",
				Content:      "hello",
			},
		),
	)

	assert.NoError(tx.Commit())

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusGone,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
	}

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrActorGone))
	assert.Empty(client)

	var ok int
	assert.NoError(db.QueryRow(`select not exists (select 1 from notes where author = 'https://0.0.0.0/user/dan') and not exists (select 1 from persons where id = 'https://0.0.0.0/user/dan')`).Scan(&ok))
	assert.Equal(1, ok)
}

func TestResolve_FederatedActorFirstTimeWrongID(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorFirstTimeDeleted(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusGone,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/user/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/user/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	tx, err := db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
			tx,
			&ap.Object{
				ID:           "https://0.0.0.0/note/1",
				Type:         ap.Note,
				AttributedTo: "https://0.0.0.0/user/dan",
				Content:      "hello",
			},
		),
	)

	assert.NoError(tx.Commit())

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrActorGone))
	assert.Empty(client)

	var ok int
	assert.NoError(db.QueryRow(`select exists (select 1 from notes where author = 'https://0.0.0.0/user/dan') and not exists (select 1 from persons where id = 'https://0.0.0.0/user/dan')`).Scan(&ok))
	assert.Equal(1, ok)
}

func TestResolve_FederatedActorFirstTimeTooYoung(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	_, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client)
}

func TestResolve_FederatedActorWrongIDCached(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/users/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorWrongIDCachedOldCache(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/users/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 `)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan123",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)
}

func TestResolve_FederatedActorWrongIDOldCache(t *testing.T) {
	assert := assert.New(t)

	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	assert.NoError(err)
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open("sqlite3", path+"?_journal_mode=WAL")
	assert.NoError(err)

	blockList := BlockList{}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	client := testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	assert.NoError(migrations.Run(context.Background(), slog.Default(), "localhost.localdomain", db))

	nobody, err := user.CreateNobody(context.Background(), "localhost.localdomain", db)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client)

	actor, err := resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/users/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan123",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/user/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/users/dan'`)
	assert.NoError(err)

	client = testClient{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"aliases": [
							"https://0.0.0.0/users/dan"
						],
						"links": [
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/activity+json"
							},
							{
								"href": "https://0.0.0.0/users/dan",
								"rel": "self",
								"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
							}
						],
						"subject": "acct:dan@0.0.0.0"
					}`))),
			},
		},
		"https://0.0.0.0/users/dan": testResponse{
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body: io.NopCloser(bytes.NewReader([]byte(
					`{
						"@context": [
							"https://www.w3.org/ns/activitystreams",
							"https://w3id.org/security/v1"
						],
						"id": "https://0.0.0.0/users/dan",
						"type": "Person",
						"inbox": "https://0.0.0.0/inbox/dan456",
						"outbox": "https://0.0.0.0/outbox/dan",
						"preferredUsername": "dan",
						"followers": "https://0.0.0.0/followers/dan",
						"endpoints": {
							"sharedInbox": "https://0.0.0.0/inbox/nobody"
						}
					}`))),
			},
		},
	}

	actor, err = resolver.ResolveID(context.Background(), slog.Default(), db, nobody, "https://0.0.0.0/users/dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan456", actor.Inbox)
}
