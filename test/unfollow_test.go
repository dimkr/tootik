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

func TestUnfollow_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	say := server.Handle("/users/whisper?Hello%20followers", server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello followers")

	unfollow := server.Handle(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), unfollow)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(t, today, "Hello followers")
}

func TestUnfollow_FollowAgain(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	say := server.Handle("/users/whisper?Hello%20followers", server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello followers")

	unfollow := server.Handle(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), unfollow)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(t, today, "Hello followers")

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello followers")
}

func TestUnfollow_NotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	unfollow := server.Handle(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, "40 No such follow\r\n", unfollow)
}

func TestUnfollow_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	unfollow := server.Handle(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(server.Bob.ID))), nil)
	assert.Equal(t, "30 /users\r\n", unfollow)
}
