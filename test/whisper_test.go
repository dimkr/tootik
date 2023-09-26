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

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	view := server.Handle(whisper[3:len(whisper)-2], server.Bob)
	assert.Contains(t, view, "Hello world")

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(t, outbox, "Hello world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(t, local, "Hello world")
}

func TestWhisper_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	view := server.Handle(whisper[3:len(whisper)-2], server.Bob)
	assert.Contains(t, view, "Hello world")

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Alice)
	assert.Contains(t, outbox, "Hello world")

	whisper = server.Handle("/users/whisper?Hello%20once%20more,%20world", server.Alice)
	assert.Equal(t, "40 Please wait before posting again\r\n", whisper)

	outbox = server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(t, outbox, "Hello world")
	assert.NotContains(t, outbox, "Hello once more, world")

	local := server.Handle("/local", server.Carol)
	assert.NotContains(t, local, "Hello world")
	assert.NotContains(t, local, "Hello once more, world")
}
