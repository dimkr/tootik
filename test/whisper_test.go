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

func TestWhisper_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	view := server.Handle(whisper[3:len(whisper)-2], server.Bob)
	assert.Contains(view, "Hello world")

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(outbox, "Hello world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(local, "Hello world")
}

func TestWhisper_FollowAfterPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	view := server.Handle(whisper[3:len(whisper)-2], server.Bob)
	assert.Equal("40 Post not found\r\n", view)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	view = server.Handle(whisper[3:len(whisper)-2], server.Bob)
	assert.Contains(view, "Hello world")

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(outbox, "Hello world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(local, "Hello world")
}

func TestWhisper_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	view := server.Handle(whisper[3:len(whisper)-2], server.Bob)
	assert.Contains(view, "Hello world")

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Alice)
	assert.Contains(outbox, "Hello world")

	whisper = server.Handle("/users/whisper?Hello%20once%20more,%20world", server.Alice)
	assert.Equal("40 Please wait before posting again\r\n", whisper)

	outbox = server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(outbox, "Hello world")
	assert.NotContains(outbox, "Hello once more, world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(local, "Hello world")
	assert.NotContains(local, "Hello once more, world")
}
