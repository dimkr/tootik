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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/stretchr/testify/assert"
)

func TestForward_ReplyToPostByFollower(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_ReplyToPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	cc.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
				CC:           cc,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_LocalReplyToLocalPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Alice", say[15:len(say)-2]), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity->>'$.type' = 'Create' and activity->>'$.object.id' = 'https://' || ? and sender = ?)`, reply[15:len(reply)-2], server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_ReplyToReplyToPostByFollower(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/2",
				Type:         ap.Note,
				AttributedTo: server.Bob.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/1",
				Content:      "hola",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/2","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_ReplyToUnknownPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/3","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?))`, reply).Scan(&forwarded))
	assert.Equal(0, forwarded)
}

func TestForward_ReplyToDM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Bob.ID)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?))`, reply).Scan(&forwarded))
	assert.Equal(0, forwarded)
}

func TestForward_NotFollowingAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_NotReplyToLocalPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://127.0.0.1/note/2","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?))`, reply).Scan(&forwarded))
	assert.Equal(0, forwarded)
}

func TestForward_ReplyToFederatedPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add("https://127.0.0.1/followers/erin")

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://127.0.0.1/note/1",
				Type:         ap.Note,
				AttributedTo: "https://127.0.0.1/user/erin",
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://127.0.0.1/note/1","content":"bye","to":["https://127.0.0.1/user/erin"],"cc":["https://127.0.0.1/followers/erin"]},"to":["https://127.0.0.1/user/erin"],"cc":["https://127.0.0.1/followers/erin"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?))`, reply).Scan(&forwarded))
	assert.Equal(0, forwarded)
}

func TestForward_MaxDepth(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/2",
				Type:         ap.Note,
				AttributedTo: server.Bob.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/1",
				Content:      "hola",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/3",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/2",
				Content:      "hi",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/4",
				Type:         ap.Note,
				AttributedTo: server.Bob.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/3",
				Content:      "hiii",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/4","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_MaxDepthPlusOne(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/2",
				Type:         ap.Note,
				AttributedTo: server.Bob.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/1",
				Content:      "hola",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/3",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/2",
				Content:      "hi",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/4",
				Type:         ap.Note,
				AttributedTo: server.Bob.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/3",
				Content:      "hiii",
				To:           to,
			},
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/5",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				InReplyTo:    "https://localhost.localdomain:8443/note/4",
				Content:      "byeee",
				To:           to,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/5","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(0, forwarded)
}

func TestForward_ReplyToLocalPostByLocalFollower(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	whisper := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Alice", whisper[15:len(whisper)-2]), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity->>'type' = 'Create' and activity->>'$.object.id' = 'https://' || ? and sender = ?)`, reply[15:len(reply)-2], server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_EditedReplyToLocalPostByLocalFollower(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	whisper := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Alice", whisper[15:len(whisper)-2]), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	id := reply[15 : len(reply)-2]

	server.cfg.EditThrottleUnit = 0

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Welcome%%20%%40alice", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity->>'type' = 'Update' and activity->>'$.object.id' = 'https://' || ? and sender = ?)`, id, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_DeletedReplyToLocalPostByLocalFollower(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	whisper := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Alice", whisper[15:len(whisper)-2]), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	id := reply[15 : len(reply)-2]

	server.cfg.EditThrottleUnit = 0

	delete := server.Handle(fmt.Sprintf("/users/delete/%s?Welcome%%20%%40alice", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), delete)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity->>'$.type' = 'Delete' and activity->>'$.object.id' = 'https://' || ? and sender = ?)`, id, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_EditedReplyToPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	cc.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
				CC:           cc,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","id":"https://127.0.0.1/user/dan","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	update, err := json.Marshal(ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      "https://127.0.0.1/update/1",
		Type:    ap.Update,
		Actor:   "https://127.0.0.1/user/dan",
		Object: &ap.Object{
			ID:           "https://127.0.0.1/note/1",
			Type:         ap.Note,
			AttributedTo: "https://127.0.0.1/user/dan",
			InReplyTo:    "https://localhost.localdomain:8443/note/1",
			Content:      "bye",
			Updated:      ap.Time{Time: time.Now().Add(time.Second)},
			To:           to,
			CC:           cc,
		},
		To: to,
		CC: cc,
	})
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		string(update),
	)
	assert.NoError(err)

	n, err = server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, string(update), server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_ResentEditedReplyToPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	cc.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
				CC:           cc,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","id":"https://127.0.0.1/user/dan","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	update, err := json.Marshal(ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      "https://127.0.0.1/update/1",
		Type:    ap.Update,
		Actor:   "https://127.0.0.1/user/dan",
		Object: &ap.Object{
			ID:           "https://127.0.0.1/note/1",
			Type:         ap.Note,
			AttributedTo: "https://127.0.0.1/user/dan",
			InReplyTo:    "https://localhost.localdomain:8443/note/1",
			Content:      "bye",
			Updated:      ap.Time{Time: time.Now().Add(time.Second)},
			To:           to,
			CC:           cc,
		},
		To: to,
		CC: cc,
	})
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		string(update),
	)
	assert.NoError(err)

	n, err = server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		string(update),
	)
	assert.NoError(err)

	n, err = server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity = jsonb(?) and sender = ?`, string(update), server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_DeletedReplyToPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	cc.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
				CC:           cc,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","id":"https://127.0.0.1/user/dan","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	delete := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/delete/1","type":"Delete","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note"},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		delete,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(2, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = jsonb(?) and sender = ?)`, delete, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_DeletedDeletedReplyToPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	cc.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		server.inbox.Accept(
			context.Background(),
			server.Alice,
			httpsig.Key{},
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "hello",
				To:           to,
				CC:           cc,
			},
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","id":"https://127.0.0.1/user/dan","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		reply,
	)
	assert.NoError(err)

	delete := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/delete/1","type":"Delete","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note"},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		delete,
	)
	assert.NoError(err)

	n, err := server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(2, n)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		delete,
	)
	assert.NoError(err)

	n, err = server.queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity = jsonb(?) and sender = ?`, delete, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}
