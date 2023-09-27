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

func TestFollows_NoFollows(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follows := server.Handle("/users/follows", server.Alice)
	assert.Contains(t, follows, "No followed users.")
}

func TestFollows_TwoInactive(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follows := server.Handle("/users/follows", server.Alice)
	assert.Contains(t, follows, server.Bob.PreferredUsername)
	assert.NotContains(t, follows, server.Carol.PreferredUsername)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	follows = server.Handle("/users/follows", server.Alice)
	assert.Contains(t, follows, server.Bob.PreferredUsername)
	assert.Contains(t, follows, server.Carol.PreferredUsername)
}

func TestFollows_OneActiveOneInactive(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	follows := server.Handle("/users/follows", server.Alice)
	assert.Contains(t, follows, server.Bob.PreferredUsername)
	assert.NotContains(t, follows, "1 post")
	assert.Contains(t, follows, server.Carol.PreferredUsername)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	follows = server.Handle("/users/follows", server.Alice)
	assert.Contains(t, follows, server.Bob.PreferredUsername)
	assert.Contains(t, follows, "1 post")
	assert.Contains(t, follows, server.Carol.PreferredUsername)
}

func TestFollows_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follows := server.Handle("/users/follows", nil)
	assert.Equal(t, "30 /users\r\n", follows)
}
