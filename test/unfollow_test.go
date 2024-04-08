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

func TestUnfollow_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	postPublic := server.Handle("/users/post/followers?Hello%20followers", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello followers")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Hello followers")
}

func TestUnfollow_FollowAgain(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	postPublic := server.Handle("/users/post/followers?Hello%20followers", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello followers")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Hello followers")

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello followers")
}

func TestUnfollow_NotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal("40 No such follow\r\n", unfollow)
}

func TestUnfollow_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), nil)
	assert.Equal("30 /users\r\n", unfollow)
}
