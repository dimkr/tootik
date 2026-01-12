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
	"io"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/migrations"
	"github.com/stretchr/testify/assert"
)

func TestDeliver_TwoUsersTwoPosts(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	bob, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "bob", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/bob')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/2","type":"Create","actor":"https://localhost.localdomain/user/bob","object":{"id":"https://localhost.localdomain/note/2","type":"Note","attributedTo":"https://localhost.localdomain/user/bob","content":"bye","inReplyTo":"https://localhost.localdomain/note/1","to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]},"to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		reply,
		bob.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_ForwardedPost(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	bob, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "bob", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/bob')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		bob.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_OneFailed(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	bob, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "bob", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/bob')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/2","type":"Create","actor":"https://localhost.localdomain/user/bob","object":{"id":"https://localhost.localdomain/note/2","type":"Note","attributedTo":"https://localhost.localdomain/user/bob","content":"bye","inReplyTo":"https://localhost.localdomain/note/1","to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]},"to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		reply,
		bob.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_OneFailedRetry(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	cfg.DeliveryRetryInterval = 0

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
}

func TestDeliver_OneInvalidURLRetry(t *testing.T) {
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
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes:inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	cfg.DeliveryRetryInterval = 0

	client.Data = map[string]testResponse{}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
}

func TestDeliver_MaxAttempts(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	cfg.DeliveryRetryInterval = 0
	cfg.MaxDeliveryAttempts = 2

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
}

func TestDeliver_SharedInbox(t *testing.T) {
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
		"https://ip6-allnodes/inbox/nobody": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan","endpoints":{"sharedInbox":"https://ip6-allnodes/inbox/nobody"}}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin","endpoints":{"sharedInbox":"https://ip6-allnodes/inbox/nobody"}}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/frank",
		`{"type":"Person","id":"https://ip6-allnodes/user/frank","preferredUsername":"frank","inbox":"https://ip6-allnodes/inbox/frank"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/frank', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_SharedInboxRetry(t *testing.T) {
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
		"https://ip6-allnodes/inbox/nobody": {
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan","endpoints":{"sharedInbox":"https://ip6-allnodes/inbox/nobody"}}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin","endpoints":{"sharedInbox":"https://ip6-allnodes/inbox/nobody"}}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/frank",
		`{"type":"Person","id":"https://ip6-allnodes/user/frank","preferredUsername":"frank","inbox":"https://ip6-allnodes/inbox/frank"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/frank', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/nobody": {
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	cfg.DeliveryRetryInterval = 0

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/nobody": {
			Response: &http.Response{
				StatusCode: http.StatusInternalServerError,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
}

func TestDeliver_SharedInboxUnknownActor(t *testing.T) {
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
		"https://ip6-allnodes/inbox/nobody": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan","endpoints":{"sharedInbox":"https://ip6-allnodes/inbox/nobody"}}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/frank",
		`{"type":"Person","id":"https://ip6-allnodes/user/frank","preferredUsername":"frank","inbox":"https://ip6-allnodes/inbox/frank"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/frank', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	cfg.DeliveryRetryInterval = 0

	client.Data = map[string]testResponse{}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_SharedInboxSingleWorker(t *testing.T) {
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
	cfg.DeliveryWorkers = 1

	client := newTestClient(map[string]testResponse{
		"https://ip6-allnodes/inbox/nobody": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan","endpoints":{"sharedInbox":"https://ip6-allnodes/inbox/nobody"}}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin","endpoints":{"sharedInbox":"https://ip6-allnodes/inbox/nobody"}}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/frank",
		`{"type":"Person","id":"https://ip6-allnodes/user/frank","preferredUsername":"frank","inbox":"https://ip6-allnodes/inbox/frank"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/frank', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_SameInbox(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/frank"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/frank",
		`{"type":"Person","id":"https://ip6-allnodes/user/frank","preferredUsername":"frank","inbox":"https://ip6-allnodes/inbox/frank"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/frank', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice"],"cc":[]},"to":["https://localhost.localdomain/followers/alice"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_ToAndCCDuplicates(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	bob, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "bob", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/bob')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://localhost.localdomain/followers/alice","https://ip6-allnodes/user/erin"],"cc":["https://ip6-allnodes/user/dan","https://ip6-allnodes/user/erin"]},"to":["https://localhost.localdomain/followers/alice","https://ip6-allnodes/user/erin"],"cc":["https://ip6-allnodes/user/dan","https://ip6-allnodes/user/erin"]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/2","type":"Create","actor":"https://localhost.localdomain/user/bob","object":{"id":"https://localhost.localdomain/note/2","type":"Note","attributedTo":"https://localhost.localdomain/user/bob","content":"bye","inReplyTo":"https://localhost.localdomain/note/1","to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]},"to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		reply,
		bob.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_PublicInTo(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	bob, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "bob", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/bob')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":[]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/2","type":"Create","actor":"https://localhost.localdomain/user/bob","object":{"id":"https://localhost.localdomain/note/2","type":"Note","attributedTo":"https://localhost.localdomain/user/bob","content":"bye","inReplyTo":"https://localhost.localdomain/note/1","to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]},"to":["https://localhost.localdomain/user/alice","https://localhost.localdomain/followers/bob"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		reply,
		bob.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}

func TestDeliver_AuthorInTo(t *testing.T) {
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
		"https://ip6-allnodes/inbox/dan": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	})

	assert.NoError(migrations.Run(context.Background(), "localhost.localdomain", db))

	alice, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "alice", nil)
	assert.NoError(err)

	bob, _, err := user.Create(context.Background(), "localhost.localdomain", db, &cfg, "bob", nil)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/dan",
		`{"type":"Person","id":"https://ip6-allnodes/user/dan","preferredUsername":"dan","inbox":"https://ip6-allnodes/inbox/dan"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://ip6-allnodes/user/erin",
		`{"type":"Person","id":"https://ip6-allnodes/user/erin","preferredUsername":"erin","inbox":"https://ip6-allnodes/inbox/erin"}`,
	)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/1', 'https://ip6-allnodes/user/dan', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/2', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/alice')`)
	assert.NoError(err)

	_, err = db.Exec(`INSERT INTO follows(id, follower, inserted, accepted, followed) VALUES ('https://ip6-allnodes/follow/3', 'https://ip6-allnodes/user/erin', UNIXEPOCH() - 5, 1, 'https://localhost.localdomain/user/bob')`)
	assert.NoError(err)

	resolver := NewResolver(&blockList, "localhost.localdomain", &cfg, &client, db)

	q := Queue{
		Domain:   "localhost.localdomain",
		Config:   &cfg,
		DB:       db,
		Resolver: resolver,
	}

	post := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/1","type":"Create","actor":"https://localhost.localdomain/user/alice","object":{"id":"https://localhost.localdomain/note/1","type":"Note","attributedTo":"https://localhost.localdomain/user/alice","content":"hello","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":[]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		post,
		alice.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)

	reply := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://localhost.localdomain/create/2","type":"Create","actor":"https://localhost.localdomain/user/bob","object":{"id":"https://localhost.localdomain/note/2","type":"Note","attributedTo":"https://localhost.localdomain/user/bob","content":"bye","inReplyTo":"https://localhost.localdomain/note/1","to":["https://localhost.localdomain/user/bob","https://localhost.localdomain/followers/bob"],"cc":[]},"to":["https://localhost.localdomain/user/bob","https://localhost.localdomain/followers/bob"],"cc":[]}`

	_, err = db.Exec(
		`INSERT INTO outbox (activity, sender, inserted) VALUES (?,?,?)`,
		reply,
		bob.ID,
		time.Now().UnixNano(),
	)
	assert.NoError(err)

	client.Data = map[string]testResponse{
		"https://ip6-allnodes/inbox/erin": {
			Response: &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader([]byte(`{}`))),
			},
		},
	}

	_, err = q.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Empty(client.Data)
}
