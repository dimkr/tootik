/*
Copyright 2024, 2025 Dima Krasner

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
	"net/http"
	"strings"
	"testing"
	"time"

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
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Create' and activity->>'$.object.id' = 'https://' || ? and activity->>'$.actor' = ? and sender = ?`, id, server.Bob.ID, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://' || $2 and sender = $1`, server.Alice.ID, id).Scan(&shared))
	assert.Equal(1, shared)
}

func TestCommunity_NewThreadNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Create' and activity->>'$.object.id' = 'https://' || ? and activity->>'$.actor' = ? and sender = ?`, id, server.Bob.ID, server.Alice.ID).Scan(&forwarded))
	assert.Equal(0, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://' || $2 and sender = $1`, server.Alice.ID, id).Scan(&shared))
	assert.Equal(0, shared)
}

func TestCommunity_NewThreadNotPublic(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Create' and activity->>'$.object.id' = 'https://' || ? and activity->>'$.actor' = ? and sender = ?`, id, server.Bob.ID, server.Alice.ID).Scan(&forwarded))
	assert.Equal(0, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://' || $2 and sender = $1`, server.Alice.ID, id).Scan(&shared))
	assert.Equal(0, shared)
}

func TestCommunity_ReplyInThread(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	to := ap.Audience{}
	to.Add(ap.Public)
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
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		&reply,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Keys:      server.NobodyKeys,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity = jsonb(?) and sender = ?`, &reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://127.0.0.1/note/1' and sender = $1`, server.Alice.ID).Scan(&shared))
	assert.Equal(1, shared)
}

func TestCommunity_ReplyInThreadAuthorNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	to := ap.Audience{}
	to.Add(ap.Public)
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
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		&reply,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Keys:      server.NobodyKeys,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity = jsonb(?) and sender = ?`, &reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(0, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://127.0.0.1/note/1' and sender = $1`, server.Alice.ID).Scan(&shared))
	assert.Equal(0, shared)
}

func TestCommunity_ReplyInThreadSenderNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/erin",
		`{"type":"Person","preferredUsername":"erin"}`,
	)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	to := ap.Audience{}
	to.Add(ap.Public)
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
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/erin",
		&reply,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Keys:      server.NobodyKeys,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity = jsonb(?) and sender = ?`, &reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://127.0.0.1/note/1' and sender = $1`, server.Alice.ID).Scan(&shared))
	assert.Equal(1, shared)
}

func TestCommunity_DuplicateReplyInThread(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	to := ap.Audience{}
	to.Add(ap.Public)
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
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		&reply,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Keys:      server.NobodyKeys,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		&reply,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity = jsonb(?) and sender = ?`, &reply, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://127.0.0.1/note/1' and sender = $1`, server.Alice.ID).Scan(&shared))
	assert.Equal(1, shared)
}

func TestCommunity_EditedReplyInThread(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	to := ap.Audience{}
	to.Add(ap.Public)
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
			Published:    ap.Time{Time: time.Now().Add(-time.Hour * 24)},
		},
	}

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		&reply,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Keys:      server.NobodyKeys,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Create' and activity->>'$.object.id' = 'https://' || ? and activity->>'$.actor' = ? and sender = ?`, id, server.Bob.ID, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.actor' = $1 and activity->>'$.object' = 'https://' || $2 and sender = $1`, server.Alice.ID, id).Scan(&shared))
	assert.Equal(1, shared)

	update := ap.Activity{
		Context: []string{"https://www.w3.org/ns/activitystreams"},
		ID:      "https://127.0.0.1/update/1",
		Type:    ap.Update,
		Actor:   "https://127.0.0.1/user/dan",
		Object: &ap.Object{
			ID:           "https://127.0.0.1/note/1",
			Type:         ap.Note,
			AttributedTo: "https://127.0.0.1/user/dan",
			InReplyTo:    "https://" + id,
			Content:      "bye",
			To:           to,
			Published:    ap.Time{Time: time.Now().Add(-time.Hour * 24)},
			Updated:      ap.Time{Time: time.Now().Add(time.Hour)},
		},
	}

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		&update,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity = jsonb(?) and sender = ?`, &update, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)
}

func TestCommunity_UnknownEditedReplyInThread(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`update persons set actor = jsonb_set(actor, '$.type', 'Group') where id = $1`,
		server.Alice.ID,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values (?, jsonb(?))`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	tx, err := server.db.BeginTx(t.Context(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	assert.NoError(
		outbox.Accept(
			context.Background(),
			domain,
			server.Alice.ID,
			"https://127.0.0.1/user/dan",
			"https://localhost.localdomain:8443/follow/1",
			tx,
		),
	)

	assert.NoError(tx.Commit())

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	to := ap.Audience{}
	to.Add(ap.Public)
	to.Add(server.Bob.ID)

	update := ap.Activity{
		Context: []string{"https://www.w3.org/ns/activitystreams"},
		ID:      "https://127.0.0.1/update/1",
		Type:    ap.Update,
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
		`insert into inbox (sender, activity, raw) values ($1, jsonb($2), $2)`,
		"https://127.0.0.1/user/dan",
		&update,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Keys:      server.NobodyKeys,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	var forwarded int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Update' and activity->>'$.object.id' = 'https://127.0.0.1/note/1' and activity->>'$.actor' = 'https://127.0.0.1/user/dan' and sender = ?`, server.Alice.ID).Scan(&forwarded))
	assert.Equal(1, forwarded)

	var shared int
	assert.NoError(server.db.QueryRow(`select count(*) from outbox where activity->>'$.type' = 'Announce' and activity->>'$.object' = 'https://127.0.0.1/note/1' and sender = ?`, server.Alice.ID).Scan(&shared))
	assert.Equal(1, shared)
}
