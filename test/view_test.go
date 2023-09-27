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
	"crypto/sha256"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestView_NoReplies(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")
}

func TestView_OneReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")
}

func TestView_TwoReplies(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")
	assert.Contains(t, view, "Welcome, Bob!")
}

func TestView_TwoRepliesBigOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle(fmt.Sprintf("/users/view/%s?123", hash), server.Alice)
	assert.NotContains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")
	assert.NotContains(t, view, "Welcome, Bob!")
}

func TestView_TwoRepliesBigOffsetUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle(fmt.Sprintf("/view/%s?123", hash), nil)
	assert.NotContains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")
	assert.NotContains(t, view, "Welcome, Bob!")
}

func TestView_TwoRepliesUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?Welcome,%%20Bob%%21", hash), server.Carol)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view := server.Handle("/view/"+hash, nil)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")
	assert.Contains(t, view, "Welcome, Bob!")
}

func TestView_OneReplyPostDeleted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	delete := server.Handle("/users/delete/"+hash, server.Bob)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), delete)

	view = server.Handle("/users/view/"+replyHash, server.Alice)
	assert.Contains(t, view, "Welcome Bob")
}

func TestView_OneReplyPostNotDeleted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	view = server.Handle("/users/view/"+replyHash, server.Alice)
	assert.Contains(t, view, "Welcome Bob")
}

func TestView_OneReplyPostNotDeletedUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/view/"+hash, nil)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	view = server.Handle("/view/"+replyHash, nil)
	assert.Contains(t, view, "Welcome Bob")
}

func TestView_OneReplyPostDeletedUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/view/"+hash, nil)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	delete := server.Handle("/users/delete/"+hash, server.Bob)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), delete)

	view = server.Handle("/view/"+replyHash, nil)
	assert.Contains(t, view, "Welcome Bob")
}

func TestView_OneReplyReplyDeleted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	delete := server.Handle("/users/delete/"+replyHash, server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), delete)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
}

func TestView_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	view := server.Handle("/users/view/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7", server.Bob)
	assert.Equal(t, "40 Post not found\r\n", view)
}

func TestView_InvalidOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	view := server.Handle("/users/view/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7?z", server.Bob)
	assert.Equal(t, "40 Invalid query\r\n", view)
}
