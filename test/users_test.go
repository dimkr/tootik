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

func TestUsers_NoFollows(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Bob)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
}

func TestUsers_NewPublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")

	users = server.Handle("/users", server.Carol)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	local := server.Handle("/users/local", server.Carol)
	assert.Contains(local, "Hello world")
}

func TestUsers_NewPostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")

	users = server.Handle("/users", server.Carol)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	local := server.Handle("/users/local", server.Carol)
	assert.NotContains(local, "Hello world")
}

func TestUsers_NewPostInFollowedGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/people",
		"ceccc92a1b91b925111b9a8a2bb27b6e0a4c0e4e56d1061c6bfde5deb9151a26",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/ceccc92a1b91b925111b9a8a2bb27b6e0a4c0e4e56d1061c6bfde5deb9151a26", server.Alice)
	assert.Equal("30 /users/outbox/ceccc92a1b91b925111b9a8a2bb27b6e0a4c0e4e56d1061c6bfde5deb9151a26\r\n", follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people in @people@other.localdomain")

	users = server.Handle("/users", server.Carol)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	local := server.Handle("/users/local", server.Carol)
	assert.NotContains(local, "Hello people in @people@other.localdomain")
}

func TestUsers_NewDM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "No posts.")
	assert.NotContains(today, "Hello Alice")

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20Alice", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", dm)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "No posts.")
	assert.Contains(today, "Hello Alice")

	users = server.Handle("/users", server.Carol)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	local := server.Handle("/users/local", server.Carol)
	assert.NotContains(local, "Hello Alice")
}

func TestUsers_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", nil)
	assert.Equal("61 Peer certificate is required\r\n", users)
}
