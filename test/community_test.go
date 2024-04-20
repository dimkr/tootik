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
	"context"
	"log/slog"
	"net/http"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/outbox"
	"github.com/stretchr/testify/assert"
)

func TestCommunity_NewThread(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = json_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			server.db,
		),
	)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity->>'$.type' = 'Announce' and activity->>'$.object.type' = 'Create' and activity->>'$.object.object.id' = 'https://' || $1 and activity->>'$.object.object.audience' = $2 and sender = $2)`, id, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestCommunity_ReplyInThread(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = json_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			server.db,
		),
	)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	to := ap.Audience{}
	to.Add(server.Bob.ID)

	reply := ap.Activity{
		Context: []string{"https://www.w3.org/ns/activitystreams"},
		ID:      "https://127.0.0.1/create/1",
		Type:    ap.Create,
		Actor:   "https://127.0.0.1/user/dan",
		Object: &ap.Object{
			ID:           "https://127.0.0.1/note/1",
			Type:         ap.Note,
			AttributedTo: "https://127.0.0.1/user/dan",
			InReplyTo:    "https://" + id,
			Content:      "bye",
			To:           to,
		},
	}

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		&reply,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		Log:       slog.Default(),
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity->>'$.type' = 'Announce' and activity->>'$.object.type' = 'Create' and activity->>'$.object.object.id' = 'https://127.0.0.1/note/1' and activity->>'$.object.object.audience' = $1 and sender = $1)`, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}
