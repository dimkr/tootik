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
	"crypto/sha256"
	"fmt"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/inbox"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"testing"
)

func TestView_NoReplies(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "Hello world")
}

func TestView_OneReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")
}

func TestView_TwoReplies(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")
	assert.Contains(view, "Welcome, Bob!")
}

func TestView_TwoRepliesBigOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle(fmt.Sprintf("/users/view/%s?123", hash), server.Alice)
	assert.NotContains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")
	assert.NotContains(view, "Welcome, Bob!")
}

func TestView_TwoRepliesBigOffsetUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle(fmt.Sprintf("/view/%s?123", hash), nil)
	assert.NotContains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")
	assert.NotContains(view, "Welcome, Bob!")
}

func TestView_TwoRepliesUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/view/"+hash, nil)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")
	assert.Contains(view, "Welcome, Bob!")
}

func TestView_OneReplyPostDeleted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	delete := server.Handle("/users/delete/"+hash, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), delete)

	view = server.Handle("/users/view/"+replyHash, server.Alice)
	assert.Contains(view, "Welcome Bob")
}

func TestView_OneReplyPostNotDeleted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	view = server.Handle("/users/view/"+replyHash, server.Alice)
	assert.Contains(view, "Welcome Bob")
}

func TestView_OneReplyPostNotDeletedUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/view/"+hash, nil)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	view = server.Handle("/view/"+replyHash, nil)
	assert.Contains(view, "Welcome Bob")
}

func TestView_OneReplyPostDeletedUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/view/"+hash, nil)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	delete := server.Handle("/users/delete/"+hash, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), delete)

	view = server.Handle("/view/"+replyHash, nil)
	assert.Contains(view, "Welcome Bob")
}

func TestView_OneReplyReplyDeleted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	delete := server.Handle("/users/delete/"+replyHash, server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), delete)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(view, "Hello world")
}

func TestView_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	view := server.Handle("/users/view/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7", server.Bob)
	assert.Equal("40 Post not found\r\n", view)
}

func TestView_InvalidOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	view := server.Handle("/users/view/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7?z", server.Bob)
	assert.Equal("40 Invalid query\r\n", view)
}

func TestView_Update(t *testing.T) {
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

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello")
	assert.NotContains(view, "bye")
	assert.NotContains(view, "edited")

	update := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Update","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"bye","updated":"2099-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		update,
	)
	assert.NoError(err)

	n, err = inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view = server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.NotContains(view, "hello")
	assert.Contains(view, "bye")
	assert.Contains(view, "edited")
}

func TestView_OldUpdate(t *testing.T) {
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

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello")
	assert.NotContains(view, "bye")
	assert.NotContains(view, "edited")

	update := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Update","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"bye","updated":"2020-10-01T05:35:36Z","to":["https://www.w3.org/ns/activitystreams#Public"]},"to":["https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		update,
	)
	assert.NoError(err)

	n, err = inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	view = server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello")
	assert.NotContains(view, "bye")
	assert.NotContains(view, "edited")
}
