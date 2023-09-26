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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestHashtags_NoHashtags(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(t, view, "Hello world")

	hashtag := server.Handle("/users/hashtags", server.Bob)
	assert.NotContains(t, hashtag, "#world")
}

func TestHashtags_OneHashtagOneAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(t, view, "Hello #world")

	hashtag := server.Handle("/users/hashtags", server.Bob)
	assert.NotContains(t, hashtag, "#world")
}

func TestHashtags_OneHashtagTwoAuthors(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(t, view, "Hello #world")

	say = server.Handle("/users/say?Hello%20again,%20%23world", server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(t, view, "Hello again, #world")

	hashtag := server.Handle("/users/hashtags", server.Carol)
	assert.Contains(t, hashtag, "#world")
}

func TestHashtags_OneHashtagTwoAuthorsCaseSensitivity(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20%23worLD", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(t, view, "Hello #worLD")

	say = server.Handle("/users/say?Hello%20again,%20%23WORld", server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(t, view, "Hello again, #WORld")

	hashtag := server.Handle("/users/hashtags", server.Carol)
	assert.Contains(t, hashtag, "#worLD")
}

func TestHashtags_TwoHashtagsOneAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(t, view, "Hello #world")

	say = server.Handle("/users/say?Hello%20%23again,%20world", server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(t, view, "Hello #again, world")

	hashtag := server.Handle("/users/hashtags", server.Carol)
	assert.NotContains(t, hashtag, "#world")
	assert.NotContains(t, hashtag, "#again")
}

func TestHashtags_OneHashtagTwoAuthorsUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20%23world", server.Alice)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(t, view, "Hello #world")

	say = server.Handle("/users/say?Hello%20again,%20%23world", server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(t, view, "Hello again, #world")

	hashtag := server.Handle("/hashtags", nil)
	assert.Contains(t, hashtag, "#world")
}
