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
	"context"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFTS_Happyflow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/users/fts?world", server.Bob)
	assert.Contains(fts, "Hello world")
}

func TestFTS_HashtagWithoutHash(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/users/fts?world", server.Bob)
	assert.NotContains(fts, "Hello #world")
}

func TestFTS_HashtagWithHash(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/users/fts?%23world", server.Bob)
	assert.NotContains(fts, "Hello #world")
}

func TestFTS_HashtagWithHashAndQuotes(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/users/fts?%22%23world%22", server.Bob)
	assert.Contains(fts, "Hello #world")
}

func TestFTS_HashtagWithHashAndQuotesUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/fts?%22%23world%22", nil)
	assert.Contains(fts, "Hello #world")
}

func TestFTS_HashtagWithHashAndQuotesSecondPage(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/users/fts?%22%23world%22%20skip%2030", server.Bob)
	assert.NotContains(fts, "Hello #world")
}

func TestFTS_NoInput(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	fts := server.Handle("/users/fts?", server.Bob)
	assert.Equal("10 Query\r\n", fts)
}

func TestFTS_EmptyInput(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	fts := server.Handle("/users/fts?", server.Bob)
	assert.Equal("10 Query\r\n", fts)
}

func TestFTS_InvalidEscapeSequence(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	fts := server.Handle("/users/fts?%zzworld", server.Bob)
	assert.Equal("40 Bad input\r\n", fts)
}

func TestFTS_UnathenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/fts?world", nil)
	assert.Contains(fts, "Hello world")
}

func TestFTS_SearchByAuthorUserName(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/users/fts?alice", server.Bob)
	assert.Contains(fts, "Hello world")
}

func TestFTS_SearchByAuthorID(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	fts := server.Handle("/users/fts?%22https%3a%2f%2flocalhost.localdomain%3a8443%2fuser%2falice%22", server.Bob)
	assert.Contains(fts, "Hello world")
}

func TestFTS_SearchByMentionUserName(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	to := ap.Audience{}
	to.Add(ap.Public)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "Hello @abc",
				To:           to,
				Tag: []ap.Tag{
					{
						Type: ap.Mention,
						Name: "@abc@localhost.localdomain:8443",
						Href: server.Bob.ID,
					},
				},
			},
		),
	)

	assert.NoError(tx.Commit())

	fts := server.Handle("/users/fts?bob", server.Bob)
	assert.Contains(fts, "Hello @abc")
}

func TestFTS_SearchByMentionID(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	tx, err := server.db.BeginTx(context.Background(), nil)
	assert.NoError(err)
	defer tx.Rollback()

	to := ap.Audience{}
	to.Add(ap.Public)

	assert.NoError(
		note.Insert(
			context.Background(),
			tx,
			&ap.Object{
				ID:           "https://localhost.localdomain:8443/note/1",
				Type:         ap.Note,
				AttributedTo: server.Alice.ID,
				Content:      "Hello @abc",
				To:           to,
				Tag: []ap.Tag{
					{
						Type: ap.Mention,
						Name: "@abc@localhost.localdomain:8443",
						Href: server.Bob.ID,
					},
				},
			},
		),
	)

	assert.NoError(tx.Commit())

	fts := server.Handle("/users/fts?%22https%3a%2f%2flocalhost.localdomain%3a8443%2fuser%2fbob%22", server.Bob)
	assert.Contains(fts, "Hello @abc")
}
