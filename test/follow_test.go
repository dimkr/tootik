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
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestFollow_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

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
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Alice", id), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal("40 Already following https://localhost.localdomain:8443/user/bob\r\n", follow)
}

func TestFollow_NoSuchUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/localhost.localdomain:8443/user/erin", server.Alice)
	assert.Equal("40 No such user\r\n", follow)
}

func TestFollow_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/localhost.localdomain:8443/user/erin", nil)
	assert.Equal("30 /users\r\n", follow)
}
