/*
Copyright 2023 Dima Krasner

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
	"github.com/dimkr/tootik/inbox"
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err = inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err = inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
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

	n, err = inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view = server.Handle("/view/bc50ef0ae381c0bd8fddd856ae156bc45d83c5212669af126ea6372800f8c9d7", server.Alice)
	assert.Contains(strings.Split(view, "\n"), "## 📊 Results (10 voters)")
	assert.Contains(strings.Split(view, "\n"), "```Results graph")
	assert.Contains(strings.Split(view, "\n"), "4 █████▎   vanilla")
	assert.Contains(strings.Split(view, "\n"), "6 ████████ chocolate")
}
