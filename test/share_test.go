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

func TestShare_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	share := server.Handle("/users/share/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), share)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")
}

func TestShare_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	share := server.Handle("/users/share/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), share)

	say = server.Handle("/users/say?Hello%20world", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id = say[15 : len(say)-2]

	share = server.Handle("/users/share/"+id, server.Bob)
	assert.Equal("40 Please wait before sharing\r\n", share)
}

func TestShare_UnshareThrottling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	share := server.Handle("/users/share/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), share)

	unshare := server.Handle("/users/unshare/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), unshare)
}

func TestShare_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	share := server.Handle("/users/share/"+id, server.Bob)
	assert.Equal("40 Error\r\n", share)
}

func TestShare_Twice(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	share := server.Handle("/users/share/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), share)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	share = server.Handle("/users/share/"+id, server.Bob)
	assert.Equal("40 Error\r\n", share)
}

func TestShare_Unshare(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	share := server.Handle("/users/share/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), share)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	_, err := server.db.Exec(`update outbox set inserted = inserted = 3600 where activity->>'$.type' = 'Announce'`)
	assert.NoError(err)

	unshare := server.Handle("/users/unshare/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), unshare)

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.NotContains(outbox, "> Hello world")
}

func TestShare_ShareAfterUnshare(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	share := server.Handle("/users/share/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), share)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	_, err := server.db.Exec(`update outbox set inserted = inserted = 3600 where activity->>'$.type' = 'Announce'`)
	assert.NoError(err)

	unshare := server.Handle("/users/unshare/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), unshare)

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.NotContains(outbox, "> Hello world")

	_, err = server.db.Exec(`update outbox set inserted = inserted = 3600 where activity->>'$.type' = 'Undo'`)
	assert.NoError(err)

	share = server.Handle("/users/share/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), share)

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")
}
