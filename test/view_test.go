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
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
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
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
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

func TestView_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	say := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "Hello world")
}

func TestView_PostToFollowersPostBeforeFollow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "Hello world")
}

func TestView_PostToFollowersUnfollow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	say := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "Hello world")

	unfollow := server.Handle(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), unfollow)

	view = server.Handle("/users/view/"+hash, server.Bob)
	assert.Equal("40 Post not found\r\n", view)
}

func TestView_PostToFollowersNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Equal("40 Post not found\r\n", view)
}

func TestView_PostToFollowersWithReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Carol)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	say := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Alice", hash), server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+hash, server.Carol)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Alice")
}

func TestView_PostInGroupPublicAndGroupFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/group/people",
		"e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello @people","to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people","https://www.w3.org/ns/activitystreams#Public"],"tag":[{"type":"Mention","name":"@people","href":"https://127.0.0.1/group/people"}]},"to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people","https://www.w3.org/ns/activitystreams#Public"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	follow := server.Handle("/users/follow/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5", server.Alice)
	assert.Equal("30 /users/outbox/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello @people")
}

func TestView_PostInGroupNotPublicAndGroupFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/group/people",
		"e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello @people","to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"],"tag":[{"type":"Mention","name":"@people","href":"https://127.0.0.1/group/people"}]},"to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	follow := server.Handle("/users/follow/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5", server.Alice)
	assert.Equal("30 /users/outbox/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello @people")
}

func TestView_PostInGroupNotPublicAndGroupFollowedButNotAccepted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/group/people",
		"e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello @people","to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"],"tag":[{"type":"Mention","name":"@people","href":"https://127.0.0.1/group/people"}]},"to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	follow := server.Handle("/users/follow/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5", server.Alice)
	assert.Equal("30 /users/outbox/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5\r\n", follow)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Equal("40 Post not found\r\n", view)
}

func TestView_PostInGroupNotPublicAndAuthorFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/group/people",
		"e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello @people","to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"],"tag":[{"type":"Mention","name":"@people","href":"https://127.0.0.1/group/people"}]},"to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	follow := server.Handle("/users/follow/eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3", server.Alice)
	assert.Equal("30 /users/outbox/eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello @people")
}

func TestView_PostInGroupNotPublicAndAuthorFollowedButNotAccepted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/group/people",
		"e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello @people","to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"],"tag":[{"type":"Mention","name":"@people","href":"https://127.0.0.1/group/people"}]},"to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(1, n)

	follow := server.Handle("/users/follow/eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3", server.Alice)
	assert.Equal("30 /users/outbox/eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3\r\n", follow)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Equal("40 Post not found\r\n", view)
}

func TestView_PostInGroupNotPublicAndGroupFollowedWithReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/group/people",
		"e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/erin",
		"07ea196f07e188f8014095ee67507b8a58261adc9d977b4a330aa3849f6e8791",
		`{"type":"Person","preferredUsername":"erin","followers":"https://127.0.0.1/followers/erin"}`,
	)
	assert.NoError(err)

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello @people","to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"],"tag":[{"type":"Mention","name":"@people","href":"https://127.0.0.1/group/people"}]},"to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	create = `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/2","type":"Create","actor":"https://127.0.0.1/user/erin","object":{"id":"https://127.0.0.1/note/2","type":"Note","attributedTo":"https://127.0.0.1/user/erin","inReplyTo":"https://127.0.0.1/note/1","content":"hello dan","to":["https://127.0.0.1/user/dan","https://127.0.0.1/followers/erin"],"cc":["https://127.0.0.1/group/people"]},"to":["https://127.0.0.1/user/dan","https://127.0.0.1/followers/erin"],"cc":["https://127.0.0.1/group/people"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(2, n)

	follow := server.Handle("/users/follow/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5", server.Alice)
	assert.Equal("30 /users/outbox/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello @people")
	assert.Contains(view, "hello dan")
}

func TestView_PostInGroupNotPublicAndGroupFollowedWithPrivateReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/group/people",
		"e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/dan",
		"eab50d465047c1ccfc581759f33612c583486044f5de62b2a5e77e220c2f1ae3",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://127.0.0.1/user/erin",
		"07ea196f07e188f8014095ee67507b8a58261adc9d977b4a330aa3849f6e8791",
		`{"type":"Person","preferredUsername":"erin","followers":"https://127.0.0.1/followers/erin"}`,
	)
	assert.NoError(err)

	create := `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"hello @people","to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"],"tag":[{"type":"Mention","name":"@people","href":"https://127.0.0.1/group/people"}]},"to":["https://127.0.0.1/followers/dan"],"cc":["https://127.0.0.1/group/people"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	create = `{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/2","type":"Create","actor":"https://127.0.0.1/user/erin","object":{"id":"https://127.0.0.1/note/2","type":"Note","attributedTo":"https://127.0.0.1/user/erin","inReplyTo":"https://127.0.0.1/note/1","content":"hello dan","to":["https://127.0.0.1/user/dan","https://127.0.0.1/followers/erin"]},"to":["https://127.0.0.1/user/dan","https://127.0.0.1/followers/erin"]}`

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/dan",
		create,
	)
	assert.NoError(err)

	n, err := inbox.ProcessBatch(context.Background(), slog.Default(), server.db, fed.NewResolver(nil), server.Nobody)
	assert.NoError(err)
	assert.Equal(2, n)

	follow := server.Handle("/users/follow/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5", server.Alice)
	assert.Equal("30 /users/outbox/e5a24abd97f88c23a66b1873f2d5837f82f03089914f293cc9301f0e0a5c38f5\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	view := server.Handle("/users/view/ff2b86e2dbb0cc086c97f1cf9b4398c26959821cddafdcd387c4471e6ec8cd65", server.Alice)
	assert.Contains(view, "hello @people")
	assert.NotContains(view, "hello dan")
}
