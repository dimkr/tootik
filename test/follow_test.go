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

func TestFollow_PostToFollowers(t *testing.T) {
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
}

func TestFollow_PostToFollowersBeforeFollow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestFollow_DMUnfollowFollow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20Alice", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")

	unfollow := server.Handle(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), unfollow)

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello Alice")
}

func TestFollow_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestFollow_Mutual(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Alice", hash), server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	users = server.Handle("/users", server.Bob)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(today, "Hello world")
}

func TestFollow_AlreadyFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal("40 Already following https://localhost.localdomain:8443/user/bob\r\n", follow)
}

func TestFollow_NoSuchUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7", server.Alice)
	assert.Equal("40 No such user\r\n", follow)
}

func TestFollow_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7", nil)
	assert.Equal("30 /users\r\n", follow)
}
