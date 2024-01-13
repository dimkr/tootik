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
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/outbox"
	"github.com/stretchr/testify/assert"
	"log/slog"
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

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://::1/user/dan",
		"2ca631feec88844591a19e830b895cadc880d127d689ff10e995750db22e07ff",
		`{"id":"https://::1/user/dan","type":"Person","alsoKnownAs":"https://127.0.0.1/user/dan"}`,
	)
	assert.NoError(err)

	assert.NoError(outbox.Move(context.Background(), domain, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) and exists (select 1 from outbox where activity->>'type' = 'Follow' and activity->>'actor' = $1 and activity->>'object' = $2)`, server.Alice.ID, "https://::1/user/dan").Scan(&followed))
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

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://::1/user/dan",
		"2ca631feec88844591a19e830b895cadc880d127d689ff10e995750db22e07ff",
		`{"id":"https://::1/user/dan","type":"Person","alsoKnownAs":["https://::1/user/dan","https://127.0.0.1/user/dan"]}`,
	)
	assert.NoError(err)

	assert.NoError(outbox.Move(context.Background(), domain, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) and exists (select 1 from outbox where activity->>'type' = 'Follow' and activity->>'actor' = $1 and activity->>'object' = $2)`, server.Alice.ID, "https://::1/user/dan").Scan(&followed))
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

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://::1/user/dan",
		"2ca631feec88844591a19e830b895cadc880d127d689ff10e995750db22e07ff",
		`{"id":"https://::1/user/dan","type":"Person"}`,
	)
	assert.NoError(err)

	assert.NoError(outbox.Move(context.Background(), domain, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) or exists (select 1 from outbox where activity->>'type' = 'Follow' and activity->>'actor' = $1 and activity->>'object' = $2)`, server.Alice.ID, "https://::1/user/dan").Scan(&followed))
	assert.Equal(0, followed)
}

func TestMove_FederatedToFederatedFollowedAfterUpdate(t *testing.T) {
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

	_, err := server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","movedTo":"https://::1/user/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://::1/user/dan",
		"2ca631feec88844591a19e830b895cadc880d127d689ff10e995750db22e07ff",
		`{"id":"https://::1/user/dan","type":"Person","alsoKnownAs":"https://127.0.0.1/user/dan"}`,
	)
	assert.NoError(err)

	assert.NoError(outbox.Move(context.Background(), domain, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 0) and exists (select 1 from outbox where activity->>'type' = 'Follow' and activity->>'actor' = $1 and activity->>'object' = $2)`, server.Alice.ID, "https://::1/user/dan").Scan(&followed))
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

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","movedTo":"https://localhost.localdomain:8443/user/bob"}`,
	)
	assert.NoError(err)

	assert.NoError(outbox.Move(context.Background(), domain, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 1) and not exists (select 1 from outbox where activity->>'type' = 'Follow' and activity->>'actor' = $1 and activity->>'object' = $2)`, server.Alice.ID, server.Bob.ID).Scan(&followed))
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

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","movedTo":"https://localhost.localdomain:8443/user/bob"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(`UPDATE persons SET actor = json_set(actor, '$.alsoKnownAs', $1) WHERE id = $2`, "https://127.0.0.1/user/dan", server.Bob.ID)
	assert.NoError(err)

	assert.NoError(outbox.Move(context.Background(), domain, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody))

	var followed int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from follows where follower = $1 and followed = $2 and accepted = 1) and not exists (select 1 from outbox where activity->>'type' = 'Follow' and activity->>'actor' = $1 and activity->>'object' = $2)`, server.Alice.ID, server.Bob.ID).Scan(&followed))
	assert.Equal(1, followed)
}
