/*
Copyright 2024 Dima Krasner

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

func TestFirehose_NoPosts(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	firehose := server.Handle("/users/firehose", server.Alice)
	assert.Contains(firehose, "No posts.")
}

func TestFirehose_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	firehose := server.Handle("/users/firehose", nil)
	assert.Equal("30 /users\r\n", firehose)
}

func TestFirehose_DM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	firehose := server.Handle("/users/firehose", server.Alice)
	assert.Contains(firehose, "Hello Alice")
}

func TestFirehose_DMNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	firehose := server.Handle("/users/firehose", server.Alice)
	assert.NotContains(firehose, "Hello Alice")
}

func TestFirehose_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	firehose := server.Handle("/users/firehose", server.Alice)
	assert.Contains(firehose, "Hello world")
}

func TestFirehose_PostToFollowersNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	firehose := server.Handle("/users/firehose", server.Alice)
	assert.NotContains(firehose, "Hello world")
}

func TestFirehose_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	firehose := server.Handle("/users/firehose", server.Alice)
	assert.Contains(firehose, "Hello world")
}

func TestFirehose_PublicPostNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	firehose := server.Handle("/users/firehose", server.Alice)
	assert.NotContains(firehose, "Hello world")
}
