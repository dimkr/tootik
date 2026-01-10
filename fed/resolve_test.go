/*
Copyright 2024 - 2026 Dima Krasner

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
	"io"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/dimkr/tootik/migrations"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
)

type testResponse struct {
	Response *http.Response
	Error    error
}

type testClient struct {
	sync.Mutex
	Data map[string]testResponse
}

func newTestResponse(statusCode int, body string) *http.Response {
	buf := []byte(body)
	return &http.Response{
		StatusCode:    statusCode,
		ContentLength: int64(len(buf)),
		Body:          io.NopCloser(bytes.NewReader(buf)),
	}
}

func newTestClient(data map[string]testResponse) testClient {
	return testClient{Data: data}
}

func (c *testClient) Do(r *http.Request) (*http.Response, error) {
	url := r.URL.String()
	c.Lock()
	resp, ok := c.Data[url]
	if !ok {
		panic("No response for " + url)
	}
	delete(c.Data, url)
	c.Unlock()
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

	client := newTestClient(map[string]testResponse{})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	app, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.ResolveID(context.Background(), key, app.ID, 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal(app.ID, actor.ID)
	assert.Equal(app.Inbox, actor.Inbox)
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

	client := newTestClient(map[string]testResponse{})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.ResolveID(context.Background(), key, "https://localhost.localdomain/user/doesnotexist", 0)
	assert.True(errors.Is(err, ErrNoLocalActor))
}

func TestResolve_FederatedInstanceActor(t *testing.T) {
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
				`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],
					"id": "https://0.0.0.0/user/dan",
					"type": "Application",
					"inbox": "https://0.0.0.0/inbox/dan",
					"outbox": "https://0.0.0.0/outbox/dan",
					"preferredUsername": "dan",
					"followers": "https://0.0.0.0/followers/dan",
					"endpoints": {
						"sharedInbox": "https://0.0.0.0/inbox/nobody"
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", ap.InstanceActor)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
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

	client := newTestClient(map[string]testResponse{})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.ResolveID(context.Background(), key, "https://0.0.0.0/user/dan%zz", 0)
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

	client := newTestClient(map[string]testResponse{})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.ResolveID(context.Background(), key, "http://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrInvalidScheme))
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.ResolveID(context.Background(), key, "https://0.0.0.0/user/dan", ap.Offline)
	assert.True(errors.Is(err, ErrActorNotCached))
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	for i := range resolver.locks {
		assert.NoError(resolver.locks[i].Lock(context.Background()))
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = resolver.ResolveID(ctx, key, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, context.Canceled))
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.Error(err)
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.ResolveID(context.Background(), key, "https://0.0.0.0/user/dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrInvalidHost))
	assert.Empty(client.Data)
}

/*
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateNobody(context.Background(), "localhost.localdomain", db,&cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.ResolveID(context.Background(), key, "https://0.0.0.0/user/dan", 0)
	assert.True(errors.Is(err, ErrInvalidHost))
	assert.Empty(client.Data)
}
*/

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://tootik.0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
				`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],
					"id": "https://tootik.0.0.0.0/user/dan",
					"type": "Person",
					"inbox": "https://tootik.0.0.0.0/inbox/dan",
					"outbox": "https://tootik.0.0.0.0/outbox/dan",
					"preferredUsername": "dan",
					"followers": "https://tootik.0.0.0.0/followers/dan",
					"endpoints": {
						"sharedInbox": "https://tootik.0.0.0.0/inbox/nobody"
					},
					"publicKey": {
						"id": "https://tootik.0.0.0.0/user/dan#main-key",
						"owner": "https://tootik.0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://tootik.0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://tootik.0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
				`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],
					"id": "https://tootik.0.0.0.0/user/dan",
					"type": "Person",
					"inbox": "https://tootik.0.0.0.0/inbox/dan",
					"outbox": "https://tootik.0.0.0.0/outbox/dan",
					"preferredUsername": "dan",
					"followers": "https://tootik.0.0.0.0/followers/dan",
					"endpoints": {
						"sharedInbox": "https://tootik.0.0.0.0/inbox/nobody"
					},
					"publicKey": {
						"id": "https://tootik.0.0.0.0/user/dan#main-key",
						"owner": "https://tootik.0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://tootik.0.0.0.0/inbox/dan", actor.Inbox)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://tootik.0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
				`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],
					"id": "https://tootik.0.0.0.0/user/dan",
					"type": "Person",
					"inbox": "https://tootik.0.0.0.0/inbox/dan",
					"outbox": "https://tootik.0.0.0.0/outbox/dan",
					"preferredUsername": "dan",
					"followers": "https://tootik.0.0.0.0/followers/dan",
					"endpoints": {
						"sharedInbox": "https://tootik.0.0.0.0/inbox/nobody"
					},
					"publicKey": {
						"id": "https://tootik.0.0.0.0/user/dan#main-key",
						"owner": "https://tootik.0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://tootik.0.0.0.0/inbox/dan", actor.Inbox)

	client.Data = map[string]testResponse{}

	actor, err = resolver.ResolveID(context.Background(), key, "https://tootik.0.0.0.0/user/dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://tootik.0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://tootik.0.0.0.0/inbox/dan", actor.Inbox)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	blockList.domains = map[string]struct{}{
		"0.0.0.0": {},
	}

	_, err = resolver.ResolveID(context.Background(), key, "https://0.0.0.0/user/dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheWasSuspended(t *testing.T) {
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					},
					"published": "2018-08-18T00:00:00Z",
					"suspended": true
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrSuspendedActor))
	assert.Empty(client.Data)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					},
					"published": "2018-08-18T00:00:00Z",
					"suspended": false
				}`,
			),
		},
	}

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					},
					"published": "2018-08-18T00:00:00Z"
				}`,
			),
		},
	}

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheUpdateFailed(t *testing.T) {
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusInternalServerError,
				`{}`,
			),
		},
	}

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					},
					"published": "2018-08-18T00:00:00Z"
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					},
					"published": "2088-08-18T00:00:00Z"
				}`,
			),
		},
	}

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					},
					"published": "2088-08-18T00:00:00Z"
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*5 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", ap.Offline)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorOldCacheExpiredDomain(t *testing.T) {
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

	client := newTestClient(map[string]testResponse{
		"https://invalid.invalid/.well-known/webfinger?resource=acct:dan@invalid.invalid": {
			Response: newTestResponse(
				http.StatusOK,
				`{
					"aliases": [
						"https://invalid.invalid/user/dan"
					],
					"links": [
						{
							"href": "https://invalid.invalid/user/dan",
							"rel": "self",
							"type": "application/activity+json"
						},
						{
							"href": "https://invalid.invalid/user/dan",
							"rel": "self",
							"type": "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
						}
					],
					"subject": "acct:dan@invalid.invalid"
				}`,
			),
		},
		"https://invalid.invalid/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
				`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],
					"id": "https://invalid.invalid/user/dan",
					"type": "Person",
					"inbox": "https://invalid.invalid/inbox/dan",
					"outbox": "https://invalid.invalid/outbox/dan",
					"preferredUsername": "dan",
					"followers": "https://invalid.invalid/followers/dan",
					"endpoints": {
						"sharedInbox": "https://invalid.invalid/inbox/nobody"
					},
					"publicKey": {
						"id": "https://invalid.invalid/user/dan#main-key",
						"owner": "https://invalid.invalid/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "invalid.invalid", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://invalid.invalid/user/dan", actor.ID)
	assert.Equal("https://invalid.invalid/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*60, fetched = unixepoch() - 60*60*60 where id = 'https://invalid.invalid/user/dan'`)
	assert.NoError(err)

	resolver.client = &http.Client{}

	_, err = resolver.Resolve(context.Background(), key, "invalid.invalid", "dan", 0)
	assert.Error(err)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrInvalidID))
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
				`abc`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
	}

	cfg.MaxResponseBodySize = 1

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
				`abc`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	cfg.MaxResponseBodySize = 419

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)
}

func TestResolve_FederatedActorFirstTimeThroughKey(t *testing.T) {
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/user/dan#main-key": {
			Response: newTestResponse(
				http.StatusOK,
				`{
					"@context": [
						"https://www.w3.org/ns/activitystreams",
						"https://w3id.org/security/v1"
					],
					"id": "https://0.0.0.0/user/dan",
					"type": "Person",
					"inbox": "https://0.0.0.0/inbox/dan",
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.ResolveID(context.Background(), key, "https://0.0.0.0/user/dan#main-key", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	actor, err = resolver.ResolveID(context.Background(), key, "https://0.0.0.0/user/dan#main-key", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Error: errors.New("a"),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Error: errors.New("a"),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/user/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	tx, err := db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
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

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/user/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/user/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusGone,
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
				}`,
			),
		},
	}

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrActorGone))
	assert.Empty(client.Data)

	var ok int
	assert.NoError(db.QueryRow(`select not exists (select 1 from notes where author = 'https://0.0.0.0/user/dan' and deleted = 0) and not exists (select 1 from persons where id = 'https://0.0.0.0/user/dan')`).Scan(&ok))
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusGone,
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
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	tx, err := db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
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

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrActorGone))
	assert.Empty(client.Data)

	var ok int
	assert.NoError(db.QueryRow(`select exists (select 1 from notes where author = 'https://0.0.0.0/user/dan' and deleted = 0) and not exists (select 1 from persons where id = 'https://0.0.0.0/user/dan')`).Scan(&ok))
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrYoungActor))
	assert.Empty(client.Data)
}

func TestResolve_FederatedActorFirstTimeSuspended(t *testing.T) {
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					},
					"published": "2018-08-18T00:00:00Z",
					"suspended": true
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	_, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.True(errors.Is(err, ErrSuspendedActor))
	assert.Empty(client.Data)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 `)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

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

	client := newTestClient(map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	_, key, err := user.CreateApplicationActor(context.Background(), "localhost.localdomain", db, &cfg)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	actor, err := resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/users/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)
	assert.Empty(client.Data)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan123", actor.Inbox)

	_, err = db.Exec(`update persons set updated = unixepoch() - 60*60*24*7, fetched = unixepoch() - 60*60*7 where id = 'https://0.0.0.0/users/dan'`)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://0.0.0.0/.well-known/webfinger?resource=acct:dan@0.0.0.0": {
			Response: newTestResponse(
				http.StatusOK,
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
				}`,
			),
		},
		"https://0.0.0.0/users/dan": {
			Response: newTestResponse(
				http.StatusOK,
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
					},
					"publicKey": {
						"id": "https://0.0.0.0/user/dan#main-key",
						"owner": "https://0.0.0.0/user/dan",
						"publicKeyPem": "abcd"
					}
				}`,
			),
		},
	}

	actor, err = resolver.Resolve(context.Background(), key, "0.0.0.0", "dan", 0)
	assert.NoError(err)

	assert.Equal("https://0.0.0.0/users/dan", actor.ID)
	assert.Equal("https://0.0.0.0/inbox/dan456", actor.Inbox)
}
