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

func TestReply_AuthorNotFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(t, users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.Contains(t, local, "Hello world")
	assert.Contains(t, local, "Welcome Bob")
}

func TestReply_AuthorFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(t, users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.Contains(t, local, "Hello world")
	assert.Contains(t, local, "Welcome Bob")
}

func TestReply_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	today := server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(t, today, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(t, local, "Hello world")
	assert.NotContains(t, local, "Welcome Bob")
}

func TestReply_SelfReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")

	server.db.Exec("update outbox set inserted = inserted - 3600 where activity->>'type' = 'Create'")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20me", hash), server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome me")

	today := server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(t, today, "Welcome me")

	local := server.Handle("/local", nil)
	assert.NotContains(t, local, "Hello world")
	assert.NotContains(t, local, "Welcome me")
}

func TestReply_ReplyToPublicPostByFollowedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Carol)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, users, "Hello world")
	assert.NotContains(t, users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(t, local, "Hello world")
	assert.NotContains(t, local, "Welcome Bob")
}

func TestReply_ReplyToPublicPostByNotFollowedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")
	assert.NotContains(t, view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", hash), server.Carol)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
	assert.Contains(t, view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(t, users, "Hello world")
	assert.NotContains(t, users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(t, local, "Hello world")
	assert.NotContains(t, local, "Welcome Bob")
}

func TestReply_DM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp(t, "^30 /users/outbox/[0-9a-f]{64}\r\n$", follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20Alice", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello Alice")
	assert.NotContains(t, today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(t, today, "Hello Alice")
	assert.NotContains(t, today, "Hello Bob")

	hash := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello Alice")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", hash), server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", reply)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello Alice")
	assert.NotContains(t, today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(t, today, "Hello Alice")
	assert.Contains(t, today, "Hello Bob")
}

func TestReply_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	reply := server.Handle("/users/reply/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7?Welcome%%20Bob", server.Alice)
	assert.Equal(t, "40 Post does not exist\r\n", reply)
}
