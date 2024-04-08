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

func TestHashtag_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	hashtag := server.Handle("/users/hashtag/world", server.Bob)
	assert.Contains(hashtag, "Hello #world")
}

func TestHashtag_PublicPostUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	hashtag := server.Handle("/hashtag/world", nil)
	assert.Contains(hashtag, "Hello #world")
}

func TestHashtag_ExclamationMark(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?Hello%20%23world%21", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "Hello #world!")

	hashtag := server.Handle("/users/hashtag/world", server.Bob)
	assert.Contains(hashtag, "Hello #world!")
}

func TestHashtag_Beginning(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?%23Hello%20world%21", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "#Hello world!")

	hashtag := server.Handle("/hashtag/Hello", server.Bob)
	assert.Contains(hashtag, "#Hello world!")
}

func TestHashtag_Multiple(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?%23Hello%20%23world%21", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "#Hello #world!")

	hashtag := server.Handle("/hashtag/Hello", server.Bob)
	assert.Contains(hashtag, "#Hello #world!")

	hashtag = server.Handle("/users/hashtag/world", server.Bob)
	assert.Contains(hashtag, "#Hello #world!")
}

func TestHashtag_CaseSensitivity(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?Hello%20%23wOrLd", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "Hello #wOrLd")

	hashtag := server.Handle("/hashtag/WoRlD", server.Bob)
	assert.Contains(hashtag, "Hello #wOrLd")
}

func TestHashtag_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	postFollowers := server.Handle("/users/post/followers?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postFollowers)

	view := server.Handle(postFollowers[3:len(postFollowers)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	hashtag := server.Handle("/users/hashtag/world", server.Bob)
	assert.NotContains(hashtag, "Hello #world")
}

func TestHashtag_BigOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	hashtag := server.Handle("/users/hashtag/world?123", server.Bob)
	assert.NotContains(hashtag, "Hello #world")
}

func TestHashtag_BigOffsetUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	hashtag := server.Handle("/hashtag/world?123", nil)
	assert.NotContains(hashtag, "Hello #world")
}

func TestHashtag_InvalidOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	postPublic := server.Handle("/users/post/public?Hello%20%23world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, postPublic)

	view := server.Handle(postPublic[3:len(postPublic)-2], server.Bob)
	assert.Contains(view, "Hello #world")

	hashtag := server.Handle("/hashtag/world?z", server.Bob)
	assert.Equal("40 Invalid query\r\n", hashtag)
}

func TestHashtag_EmptyHashtag(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	hashtag := server.Handle("/users/hashtag/", server.Bob)
	assert.Equal("30 /users/oops\r\n", hashtag)
}

func TestHashtag_EmptyHashtagUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	hashtag := server.Handle("/hashtag/", nil)
	assert.Equal("30 /oops\r\n", hashtag)
}
