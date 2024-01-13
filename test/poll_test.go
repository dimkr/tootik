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
	"crypto/sha256"
	"fmt"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/inbox"
	"github.com/dimkr/tootik/outbox"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"strings"
	"testing"
)

func TestPoll_TwoOptions(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")
}

func TestPoll_TwoOptionsZeroVotes(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":0}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":6,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (6 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "0          vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")
}

func TestPoll_TwoOptionsOnlyZeroVotes(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":0}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":0}}],"votersCount":0,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.NotContains(view, "## 📊 Results")
	assert.NotContains(strings.Split(view, "\n"), "```Results graph")
}

func TestPoll_OneOption(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}}],"votersCount":4,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (4 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "vanilla ████████ 4")
}

func TestPoll_Vote(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	reply := server.Handle("/users/reply/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7?vanilla", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	var valid int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where sender = $1 and activity->>'actor' = $1 and activity->>'object.attributedTo' = $1 and activity->>'object.type' = 'Note' and activity->>'object.inReplyTo' = 'https://127.0.0.1/poll/1' and activity->>'object.name' = 'vanilla' and activity->>'object.content' is null)`, server.Alice.ID).Scan(&valid))
	assert.Equal(1, valid)
}

func TestPoll_VoteClosedPoll(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"closed":"2020-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	reply := server.Handle("/users/reply/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7?vanilla", server.Alice)
	assert.Equal("40 Cannot vote in a closed poll\r\n", reply)

	var valid int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where sender = $1 and activity->>'actor' = $1 and activity->>'object.attributedTo' = $1 and activity->>'object.type' = 'Note' and activity->>'object.inReplyTo' = 'https://127.0.0.1/poll/1' and activity->>'object.name' = 'vanilla' and activity->>'object.content' is null)`, server.Alice.ID).Scan(&valid))
	assert.Equal(0, valid)
}

func TestPoll_VoteEndedPoll(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2020-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	reply := server.Handle("/users/reply/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7?vanilla", server.Alice)
	assert.Equal("40 Cannot vote in a closed poll\r\n", reply)

	var valid int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where sender = $1 and activity->>'actor' = $1 and activity->>'object.attributedTo' = $1 and activity->>'object.type' = 'Note' and activity->>'object.inReplyTo' = 'https://127.0.0.1/poll/1' and activity->>'object.name' = 'vanilla' and activity->>'object.content' is null)`, server.Alice.ID).Scan(&valid))
	assert.Equal(0, valid)
}

func TestPoll_Reply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	reply := server.Handle("/users/reply/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7?strawberry", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	var valid int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where sender = $1 and activity->>'actor' = $1 and activity->>'object.attributedTo' = $1 and activity->>'object.type' = 'Note' and activity->>'object.inReplyTo' = 'https://127.0.0.1/poll/1' and activity->>'object.name' is null and activity->>'object.content' = 'strawberry')`, server.Alice.ID).Scan(&valid))
	assert.Equal(1, valid)
}

func TestPoll_ReplyClosedPoll(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","closed":"2020-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	reply := server.Handle("/users/reply/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7?strawberry", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	var valid int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where sender = $1 and activity->>'actor' = $1 and activity->>'object.attributedTo' = $1 and activity->>'object.type' = 'Note' and activity->>'object.inReplyTo' = 'https://127.0.0.1/poll/1' and activity->>'object.name' is null and activity->>'object.content' = 'strawberry')`, server.Alice.ID).Scan(&valid))
	assert.Equal(1, valid)
}

func TestPoll_EditVote(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	reply := server.Handle("/users/reply/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7?vanilla", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	var valid int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where sender = $1 and activity->>'actor' = $1 and activity->>'object.attributedTo' = $1 and activity->>'object.type' = 'Note' and activity->>'object.inReplyTo' = 'https://127.0.0.1/poll/1' and activity->>'object.name' = 'vanilla' and activity->>'object.content' is null)`, server.Alice.ID).Scan(&valid))
	assert.Equal(1, valid)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?chocolate", reply[15:len(reply)-2]), server.Alice)
	assert.Equal("40 Cannot edit votes\r\n", edit)
}

func TestPoll_DeleteReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	reply := server.Handle("/users/reply/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7?strawberry", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	var valid int
	assert.NoError(server.db.QueryRow(`select exists (select 1 from outbox where sender = $1 and activity->>'actor' = $1 and activity->>'object.attributedTo' = $1 and activity->>'object.type' = 'Note' and activity->>'object.inReplyTo' = 'https://127.0.0.1/poll/1' and activity->>'object.name' is null and activity->>'object.content' = 'strawberry')`, server.Alice.ID).Scan(&valid))
	assert.Equal(1, valid)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?chocolate", reply[15:len(reply)-2]), server.Alice)
	assert.Equal("40 Please try again later\r\n", edit)
}

func TestPoll_Update(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	update := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Update","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":8}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":10}}],"votersCount":18,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		update,
	)
	assert.NoError(err)

	n, err = inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view = server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (18 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "8  ██████▍  vanilla")
	assert.Contains(strings.Split(view, "\n"), "10 ████████ chocolate")
}

func TestPoll_OldUpdate(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	update := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Update","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":8}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":10}}],"votersCount":18,"endTime":"2099-10-01T05:35:36Z","updated":"2020-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		update,
	)
	assert.NoError(err)

	n, err = inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view = server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")
}

func TestPoll_UpdateClosed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan"}`,
	)
	assert.NoError(err)

	poll := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":4}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":6}}],"votersCount":10,"endTime":"2099-10-01T05:35:36Z","closed":"2020-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		poll,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")

	update := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Update","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/poll/1","type":"Question","attributedTo":"https://127.0.0.1/user/dan","content":"vanilla or chocolate?","oneOf":[{"type":"Note","name":"vanilla","replies":{"type":"Collection","totalItems":8}},{"type":"Note","name":"chocolate","replies":{"type":"Collection","totalItems":10}}],"votersCount":18,"endTime":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		update,
	)
	assert.NoError(err)

	n, err = inbox.ProcessBatch(context.Background(), domain, server.cfg, slog.Default(), server.db, fed.NewResolver(nil, domain, server.cfg), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view = server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")
}

func TestPoll_Local3Options(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20I%20couldn%27t%20care%20less", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
}

func TestPoll_Local5Options(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20I%20couldn%27t%20care%20less%20%7c%20wut%3f%20%7c%20Maybe", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(view, "Vote wut?")
	assert.Contains(view, "Vote Maybe")
}

func TestPoll_Local1Option(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope", server.Alice)
	assert.Equal("40 Polls must have 2 to 5 options\r\n", say)
}

func TestPoll_Local6Options(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20I%20couldn%27t%20care%20less%20%7c%20wut%3f%20%7c%20Maybe%20%7c%20kinda", server.Alice)
	assert.Equal("40 Polls must have 2 to 5 options\r\n", say)
}

func TestPoll_LocalEmptyOption(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20%20%7c%20I%20couldn%27t%20care%20less", server.Alice)
	assert.Equal("40 Poll option cannot be empty\r\n", say)
}

func TestPoll_LocalOptionWithLink(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20I%20prefer%20https%3a%2f%2flocalhost%20%7c%20I%20couldn%27t%20care%20less", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote I prefer https://localhost")
	assert.Contains(view, "Vote I couldn't care less")
}

func TestPoll_Local3OptionsAnd2Votes(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20I%20couldn%27t%20care%20less", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", say[15:len(say)-2]), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?I%%20couldn%%27t%%20care%%20less", say[15:len(say)-2]), server.Carol)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view = server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
}

func TestPoll_Local3OptionsAnd2VotesAndDeletedVote(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20I%20couldn%27t%20care%20less", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", say[15:len(say)-2]), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?I%%20couldn%%27t%%20care%%20less", say[15:len(say)-2]), server.Carol)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")

	delete := server.Handle("/users/delete/"+reply[15:len(reply)-2], server.Carol)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), delete)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view = server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "0          I couldn't care less")
}

func TestPoll_LocalVoteVisibilityFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Carol)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle("/users/whisper?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20I%20couldn%27t%20care%20less", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", whisper[15:len(whisper)-2]), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?I%%20couldn%%27t%%20care%%20less", whisper[15:len(whisper)-2]), server.Carol)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view := server.Handle(whisper[3:len(whisper)-2], server.Alice)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
	assert.Contains(view, "bob")
	assert.Contains(view, "carol")

	view = server.Handle(whisper[3:len(whisper)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
	assert.Contains(view, "bob")
	assert.NotContains(view, "carol")

	view = server.Handle(whisper[3:len(whisper)-2], server.Carol)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
	assert.NotContains(view, "bob")
	assert.Contains(view, "carol")

	view = server.Handle("/view/"+whisper[15:len(whisper)-2], nil)
	assert.Equal("40 Post not found\r\n", view)
}

func TestPoll_LocalVoteVisibilityPublic(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20%7c%20I%20couldn%27t%20care%20less", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", say[15:len(say)-2]), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?I%%20couldn%%27t%%20care%%20less", say[15:len(say)-2]), server.Carol)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view := server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
	assert.Contains(view, "bob")
	assert.Contains(view, "carol")

	view = server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
	assert.Contains(view, "bob")
	assert.NotContains(view, "carol")

	view = server.Handle(say[3:len(say)-2], server.Carol)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
	assert.NotContains(view, "bob")
	assert.Contains(view, "carol")

	view = server.Handle("/view/"+say[15:len(say)-2], nil)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.NotContains(view, "Vote")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
	assert.NotContains(view, "bob")
	assert.NotContains(view, "carol")
}
