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

func TestPostFollowers_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	postFollowers := server.Handle("/users/post/followers?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postFollowers)

	view := server.Handle(postFollowers[3:len(postFollowers)-2], server.Bob)
	assert.Contains(view, "Hello world")

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Hello world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(local, "Hello world")
}

func TestPostFollowers_FollowAfterPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postFollowers := server.Handle("/users/post/followers?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postFollowers)

	view := server.Handle(postFollowers[3:len(postFollowers)-2], server.Bob)
	assert.Equal("40 Post not found\r\n", view)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	view = server.Handle(postFollowers[3:len(postFollowers)-2], server.Bob)
	assert.Contains(view, "Hello world")

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Hello world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(local, "Hello world")
}

func TestPostFollowers_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	postFollowers := server.Handle("/users/post/followers?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postFollowers)

	view := server.Handle(postFollowers[3:len(postFollowers)-2], server.Bob)
	assert.Contains(view, "Hello world")

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Alice)
	assert.Contains(outbox, "Hello world")

	postFollowers = server.Handle("/users/post/followers?Hello%20once%20more,%20world", server.Alice)
	assert.Equal("40 Please wait before posting again\r\n", postFollowers)

	outbox = server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Hello world")
	assert.NotContains(outbox, "Hello once more, world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(local, "Hello world")
	assert.NotContains(local, "Hello once more, world")
}
