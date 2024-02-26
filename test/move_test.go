/*
Copyright 2023, 2024 Dima Krasner

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
	"context"
	"fmt"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/outbox"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"net/http"
	"strings"
	"testing"
)

func TestMove_FederatedToFederated(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Alice,
			"https://127.0.0.1/user/dan",
			server.db,
		),
	)

	_, err := server.db.Exec(`update follows set accepted = 1, inserted = unixepoch()-3600`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://::1/user/dan",
		`{"id":"https://::1/user/dan","type":"Person","preferredUsername":"dan","alsoKnownAs":"https://127.0.0.1/user/dan"}`,
	)
	assert.NoError(err)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) and exists (select 1 from outbox where activity->>'$.type' = 'Follow' and activity->>'$.actor' = $1 and activity->>'$.object' = $2)`, server.Alice.ID, "https://::1/user/dan").Scan(&followed))
	assert.Equal(1, followed)
}

func TestMove_FederatedToFederatedTwoAccounts(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Alice,
			"https://127.0.0.1/user/dan",
			server.db,
		),
	)

	_, err := server.db.Exec(`update follows set accepted = 1, inserted = unixepoch()-3600`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://::1/user/dan",
		`{"id":"https://::1/user/dan","type":"Person","preferredUsername":"dan","alsoKnownAs":["https://::1/user/dan","https://127.0.0.1/user/dan"]}`,
	)
	assert.NoError(err)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) and exists (select 1 from outbox where activity->>'$.type' = 'Follow' and activity->>'$.actor' = $1 and activity->>'$.object' = $2)`, server.Alice.ID, "https://::1/user/dan").Scan(&followed))
	assert.Equal(1, followed)
}

func TestMove_FederatedToFederatedNotLinked(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Alice,
			"https://127.0.0.1/user/dan",
			server.db,
		),
	)

	_, err := server.db.Exec(`update follows set accepted = 1, inserted = unixepoch()-3600`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://::1/user/dan",
		`{"id":"https://::1/user/dan","type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) or exists (select 1 from outbox where activity->>'$.type' = 'Follow' and activity->>'$.actor' = $1 and activity->>'$.object' = $2)`, server.Alice.ID, "https://::1/user/dan").Scan(&followed))
	assert.Equal(0, followed)
}

func TestMove_FederatedToLocal(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Alice,
			"https://127.0.0.1/user/dan",
			server.db,
		),
	)

	_, err := server.db.Exec(`update follows set accepted = 1, inserted = unixepoch()-3600`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","movedTo":"https://localhost.localdomain:8443/user/bob"}`,
	)
	assert.NoError(err)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 1) and not exists (select 1 from outbox where activity->>'$.type' = 'Follow' and activity->>'$.actor' = $1 and activity->>'$.object' = $2)`, server.Alice.ID, server.Bob.ID).Scan(&followed))
	assert.Equal(0, followed)
}

func TestMove_FederatedToLocalLinked(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Alice,
			"https://127.0.0.1/user/dan",
			server.db,
		),
	)

	_, err := server.db.Exec(`update follows set accepted = 1, inserted = unixepoch()-3600`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","movedTo":"https://localhost.localdomain:8443/user/bob"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(`UPDATE persons SET actor = json_set(actor, '$.alsoKnownAs', $1) WHERE id = $2`, "https://127.0.0.1/user/dan", server.Bob.ID)
	assert.NoError(err)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 1) and not exists (select 1 from outbox where activity->>'$.type' = 'Follow' and activity->>'$.actor' = $1 and activity->>'$.object' = $2)`, server.Alice.ID, server.Bob.ID).Scan(&followed))
	assert.Equal(1, followed)
}

func TestMove_FollowingBoth(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Alice,
			"https://127.0.0.1/user/dan",
			server.db,
		),
	)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Alice,
			"https://::1/user/dan",
			server.db,
		),
	)

	_, err := server.db.Exec(`update follows set accepted = 1, inserted = unixepoch()-3600`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://::1/user/dan",
		`{"id":"https://::1/user/dan","type":"Person","preferredUsername":"dan","alsoKnownAs":"https://127.0.0.1/user/dan"}`,
	)
	assert.NoError(err)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var unfollowed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 1) and not exists (select 1 from follows where follower = $1 and followed = $3)`, server.Alice.ID, "https://::1/user/dan", "https://127.0.0.1/user/dan").Scan(&unfollowed))
	assert.Equal(1, unfollowed)
}

func TestMove_LocalToLocalAliasThrottled(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	alias := server.Handle("/users/alias?bob%40localhost.localdomain%3a8443", server.Alice)
	assert.Equal("40 Please try again later\r\n", alias)
}

func TestMove_LocalToLocal(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Carol,
			server.Alice.ID,
			server.db,
		),
	)

	server.cfg.MinActorEditInterval = 1

	alias := server.Handle("/users/alias?bob%40localhost.localdomain%3a8443", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), alias)

	alias = server.Handle("/users/alias?alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), alias)

	assert.NoError(server.db.QueryRow(`select actor from persons where id = ?`, server.Alice.ID).Scan(&server.Alice))

	move := server.Handle("/users/move?bob%40localhost.localdomain%3a8443", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), move)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var moved int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 1) and not exists (select 1 from follows where follower = $1 and followed = $3)`, server.Carol.ID, server.Bob.ID, server.Alice.ID).Scan(&moved))
	assert.Equal(1, moved)
}

func TestMove_LocalToLocalNoFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.cfg.MinActorEditInterval = 1

	alias := server.Handle("/users/alias?bob%40localhost.localdomain%3a8443", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), alias)

	alias = server.Handle("/users/alias?alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), alias)

	assert.NoError(server.db.QueryRow(`select actor from persons where id = ?`, server.Alice.ID).Scan(&server.Alice))

	move := server.Handle("/users/move?bob%40localhost.localdomain%3a8443", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), move)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var moved int
	assert.NoError(server.db.QueryRow(`select not exists (select 1 from follows where follower = $1 and followed = $2) and not exists (select 1 from follows where follower = $1 and followed = $3)`, server.Carol.ID, server.Bob.ID, server.Alice.ID).Scan(&moved))
	assert.Equal(1, moved)
}

func TestMove_LocalToFederated(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/alice",
		`{"id":"https://127.0.0.1/user/alice","type":"Person","preferredUsername":"alice","alsoKnownAs":["https://localhost.localdomain:8443/user/alice"]}`,
	)
	assert.NoError(err)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Carol,
			server.Alice.ID,
			server.db,
		),
	)

	server.cfg.MinActorEditInterval = 1

	alias := server.Handle("/users/alias?alice%40127.0.0.1", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/alice\r\n", alias)

	assert.NoError(server.db.QueryRow(`select actor from persons where id = ?`, server.Alice.ID).Scan(&server.Alice))

	move := server.Handle("/users/move?alice%40127.0.0.1", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/alice\r\n", move)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var moved int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) and not exists (select 1 from follows where follower = $1 and followed = $3)`, server.Carol.ID, "https://127.0.0.1/user/alice", server.Alice.ID).Scan(&moved))
	assert.Equal(1, moved)
}

func TestMove_LocalToFederatedNoSourceToTargetAlias(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/alice",
		`{"id":"https://127.0.0.1/user/alice","type":"Person","preferredUsername":"alice","alsoKnownAs":["https://localhost.localdomain:8443/user/alice"]}`,
	)
	assert.NoError(err)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Carol,
			server.Alice.ID,
			server.db,
		),
	)

	server.cfg.MinActorEditInterval = 1

	move := server.Handle("/users/move?alice%40127.0.0.1", server.Alice)
	assert.Equal("40 https://localhost.localdomain:8443/user/alice is not an alias for https://127.0.0.1/user/alice\r\n", move)
}

func TestMove_LocalToFederatedNoTargetToSourceAlias(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/alice",
		`{"id":"https://127.0.0.1/user/alice","type":"Person","preferredUsername":"alice","alsoKnownAs":[]}`,
	)
	assert.NoError(err)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Carol,
			server.Alice.ID,
			server.db,
		),
	)

	server.cfg.MinActorEditInterval = 1

	alias := server.Handle("/users/alias?alice%40127.0.0.1", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/alice\r\n", alias)

	assert.NoError(server.db.QueryRow(`select actor from persons where id = ?`, server.Alice.ID).Scan(&server.Alice))

	move := server.Handle("/users/move?alice%40127.0.0.1", server.Alice)
	assert.Equal("40 https://127.0.0.1/user/alice is not an alias for https://localhost.localdomain:8443/user/alice\r\n", move)
}

func TestMove_LocalToFederatedAlreadyMoved(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/alice",
		`{"id":"https://127.0.0.1/user/alice","type":"Person","preferredUsername":"alice","alsoKnownAs":["https://localhost.localdomain:8443/user/alice"]}`,
	)
	assert.NoError(err)

	assert.NoError(
		outbox.Follow(
			context.Background(),
			domain,
			server.Carol,
			server.Alice.ID,
			server.db,
		),
	)

	server.cfg.MinActorEditInterval = 1

	alias := server.Handle("/users/alias?alice%40127.0.0.1", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/alice\r\n", alias)

	assert.NoError(server.db.QueryRow(`select actor from persons where id = ?`, server.Alice.ID).Scan(&server.Alice))

	move := server.Handle("/users/move?alice%40127.0.0.1", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/alice\r\n", move)

	mover := outbox.Mover{
		Domain:   domain,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Actor:    server.Nobody,
	}
	assert.NoError(mover.Run(context.Background()))

	var moved int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) and not exists (select 1 from follows where follower = $1 and followed = $3)`, server.Carol.ID, "https://127.0.0.1/user/alice", server.Alice.ID).Scan(&moved))
	assert.Equal(1, moved)

	assert.NoError(server.db.QueryRow(`select actor from persons where id = ?`, server.Alice.ID).Scan(&server.Alice))

	move = server.Handle("/users/move?alice%40%3a%3a1", server.Alice)
	assert.Equal("40 Already moved to https://127.0.0.1/user/alice\r\n", move)
}
