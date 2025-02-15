/*
Copyright 2023 - 2025 Dima Krasner

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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHashtags_NoHashtags(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/login/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "Hello world")

	hashtag := server.Handle("/login/hashtags", server.Bob)
	assert.NotContains(strings.Split(hashtag, "\n"), "=> /login/hashtag/world #world")
}

func TestHashtags_OneHashtagOneAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/login/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	hashtag := server.Handle("/login/hashtags", server.Bob)
	assert.NotContains(strings.Split(hashtag, "\n"), "=> /login/hashtag/world #world")
}

func TestHashtags_OneHashtagTwoAuthors(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/login/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	say = server.Handle("/login/say?Hello%20again,%20%23world", server.Bob)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(view, "Hello again, #world")

	hashtag := server.Handle("/login/hashtags", server.Carol)
	assert.Contains(strings.Split(hashtag, "\n"), "=> /login/hashtag/world #world")
}

func TestHashtags_OneHashtagTwoAuthorsCaseSensitivity(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/login/say?Hello%20%23worLD", server.Alice)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "Hello #worLD")

	say = server.Handle("/login/say?Hello%20again,%20%23WORld", server.Bob)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(view, "Hello again, #WORld")

	hashtag := server.Handle("/login/hashtags", server.Carol)
	assert.Contains(strings.Split(hashtag, "\n"), "=> /login/hashtag/worLD #worLD")
}

func TestHashtags_TwoHashtagsOneAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/login/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	say = server.Handle("/login/say?Hello%20%23again,%20world", server.Bob)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(view, "Hello #again, world")

	hashtag := server.Handle("/login/hashtags", server.Carol)
	assert.NotContains(strings.Split(hashtag, "\n"), "=> /login/hashtag/world #world")
	assert.NotContains(strings.Split(hashtag, "\n"), "=> #again")
}

func TestHashtags_OneHashtagTwoAuthorsUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/login/say?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view := server.Handle(say[3:len(say)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	say = server.Handle("/login/say?Hello%20again,%20%23world", server.Bob)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	view = server.Handle(say[3:len(say)-2], server.Alice)
	assert.Contains(view, "Hello again, #world")

	hashtag := server.Handle("/hashtags", nil)
	assert.Contains(strings.Split(hashtag, "\n"), "=> /hashtag/world #world")
}
