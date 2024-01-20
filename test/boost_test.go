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

func TestBoost_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	boost := server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), boost)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")
}

func TestBoost_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	boost := server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), boost)

	say = server.Handle("/users/say?Hello%20world", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id = say[15 : len(say)-2]

	boost = server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal("40 Please wait before boosting\r\n", boost)
}

func TestBoost_UnboostThrottling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	boost := server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), boost)

	unboost := server.Handle("/users/unboost/"+id, server.Bob)
	assert.Equal("40 Please wait before unboosting\r\n", unboost)
}

func TestBoost_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	boost := server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal("40 Error\r\n", boost)
}

func TestBoost_Twice(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	boost := server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), boost)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	boost = server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal("40 Error\r\n", boost)
}

func TestBoost_Unboost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	boost := server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), boost)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	_, err := server.db.Exec(`update outbox set inserted = inserted = 3600 where activity->>'type' = 'Announce'`)
	assert.NoError(err)

	unboost := server.Handle("/users/unboost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), unboost)

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.NotContains(outbox, "> Hello world")
}

func TestBoost_BoostAfterUnboost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	boost := server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), boost)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")

	_, err := server.db.Exec(`update outbox set inserted = inserted = 3600 where activity->>'type' = 'Announce'`)
	assert.NoError(err)

	unboost := server.Handle("/users/unboost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), unboost)

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.NotContains(outbox, "> Hello world")

	_, err = server.db.Exec(`update outbox set inserted = inserted = 3600 where activity->>'type' = 'Undo'`)
	assert.NoError(err)

	boost = server.Handle("/users/boost/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), boost)

	outbox = strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Carol), "\n")
	assert.Contains(outbox, "> Hello world")
}
