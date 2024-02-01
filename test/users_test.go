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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

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

	users = server.Handle("/users", server.Carol)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	local := server.Handle("/users/local", server.Carol)
	assert.NotContains(local, "Hello world")
}

func TestUsers_NewDM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "No posts.")
	assert.NotContains(today, "Hello Alice")

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "No posts.")
	assert.Contains(today, "Hello @alice@localhost.localdomain:8443")

	users = server.Handle("/users", server.Carol)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	local := server.Handle("/users/local", server.Carol)
	assert.NotContains(local, "Hello @alice@localhost.localdomain:8443")
}

func TestUsers_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", nil)
	assert.Equal("61 Peer certificate is required\r\n", users)
}
