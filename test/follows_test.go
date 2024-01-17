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

func TestFollows_NoFollows(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follows := server.Handle("/users/follows", server.Alice)
	assert.Contains(follows, "No followed users.")
}

func TestFollows_TwoInactive(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follows := server.Handle("/users/follows", server.Alice)
	assert.Contains(follows, server.Bob.PreferredUsername)
	assert.NotContains(follows, server.Carol.PreferredUsername)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	follows = server.Handle("/users/follows", server.Alice)
	assert.Contains(follows, server.Bob.PreferredUsername)
	assert.Contains(follows, server.Carol.PreferredUsername)
}

func TestFollows_OneActiveOneInactive(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	follows := server.Handle("/users/follows", server.Alice)
	assert.Contains(follows, server.Bob.PreferredUsername)
	assert.NotContains(follows, "1 post")
	assert.Contains(follows, server.Carol.PreferredUsername)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	follows = server.Handle("/users/follows", server.Alice)
	assert.Contains(follows, server.Bob.PreferredUsername)
	assert.Contains(follows, "1 post")
	assert.Contains(follows, server.Carol.PreferredUsername)
}

func TestFollows_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follows := server.Handle("/users/follows", nil)
	assert.Equal("30 /users\r\n", follows)
}
