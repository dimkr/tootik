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
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/dimkr/tootik/outbox"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"net/http"
	"testing"
)

func TestForward_ReplyToPostByFollower(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ? and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_ReplyToPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(ap.Public)

	cc := ap.Audience{}
	cc.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ? and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_ReplyToReplyToPostByFollower(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/2","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ? and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_ReplyToUnknownPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/3","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ?)`, reply).Scan(&forwarded))
	assert.Equal(0, forwarded)
}

func TestForward_ReplyToDM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(server.Bob.ID)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ?)`, reply).Scan(&forwarded))
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
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/1","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ? and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_NotReplyToLocalPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://127.0.0.1/note/2","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/alice"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ?)`, reply).Scan(&forwarded))
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
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://127.0.0.1/note/1","content":"bye","to":["https://127.0.0.1/user/erin"],"cc":["https://127.0.0.1/followers/erin"]},"to":["https://127.0.0.1/user/erin"],"cc":["https://127.0.0.1/followers/erin"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ?)`, reply).Scan(&forwarded))
	assert.Equal(0, forwarded)
}

func TestForward_MaxDepth(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
			slog.Default(),
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
			slog.Default(),
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
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/4","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ? and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestForward_MaxDepthPlusOne(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

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

	to := ap.Audience{}
	to.Add(server.Alice.Followers)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		note.Insert(
			context.Background(),
			slog.Default(),
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
			slog.Default(),
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
			slog.Default(),
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
			slog.Default(),
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
			slog.Default(),
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
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","inReplyTo":"https://localhost.localdomain:8443/note/5","content":"bye","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://localhost.localdomain:8443/followers/bob"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		reply,
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
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where activity = ? and sender = ?)`, reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(0, forwarded)
}
