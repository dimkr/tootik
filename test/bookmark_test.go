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

func TestBookmark_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	view := strings.Split(server.Handle("/users/view/"+say[15:len(say)-2], server.Bob), "\n")
	assert.Contains(view, fmt.Sprintf("=> /users/bookmark/%s 🔖 Bookmark", say[15:len(say)-2]))
	assert.NotContains(view, fmt.Sprintf("=> /users/unbookmark/%s ✂️️ Unbookmark", say[15:len(say)-2]))

	bookmarks := strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.Contains(bookmarks, "No posts.")
	assert.NotContains(bookmarks, "> Hello world")

	bookmark := server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), bookmark)

	bookmarks = strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.NotContains(bookmarks, "No posts.")
	assert.Contains(bookmarks, "> Hello world")

	view = strings.Split(server.Handle("/users/view/"+say[15:len(say)-2], server.Bob), "\n")
	assert.NotContains(view, fmt.Sprintf("=> /users/bookmark/%s 🔖 Bookmark", say[15:len(say)-2]))
	assert.Contains(view, fmt.Sprintf("=> /users/unbookmark/%s ✂️️ Unbookmark", say[15:len(say)-2]))

	unbookmark := server.Handle("/users/unbookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), unbookmark)

	bookmarks = strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.Contains(bookmarks, "No posts.")
	assert.NotContains(bookmarks, "> Hello world")

	view = strings.Split(server.Handle("/users/view/"+say[15:len(say)-2], server.Bob), "\n")
	assert.Contains(view, fmt.Sprintf("=> /users/bookmark/%s 🔖 Bookmark", say[15:len(say)-2]))
	assert.NotContains(view, fmt.Sprintf("=> /users/unbookmark/%s ✂️️ Unbookmark", say[15:len(say)-2]))
}

func TestBookmark_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%201", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	bookmark := server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), bookmark)

	say = server.Handle("/users/say?Hello%202", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	bookmark = server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal("40 Please wait before bookmarking\r\n", bookmark)

	bookmarks := strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.NotContains(bookmarks, "No posts.")
	assert.Contains(bookmarks, "> Hello 1")
	assert.NotContains(bookmarks, "> Hello 2")
}

func TestBookmark_Limit(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%201", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	bookmark := server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), bookmark)

	say = server.Handle("/users/say?Hello%202", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	server.cfg.MinBookmarkInterval = 0
	server.cfg.MaxBookmarksPerUser = 1

	bookmark = server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal("40 Reached bookmarks limit\r\n", bookmark)

	bookmarks := strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.NotContains(bookmarks, "No posts.")
	assert.Contains(bookmarks, "> Hello 1")
	assert.NotContains(bookmarks, "> Hello 2")
}

func TestBookmark_TwoBookmarks(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%201", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	bookmark := server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), bookmark)

	say = server.Handle("/users/say?Hello%202", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	server.cfg.MinBookmarkInterval = 0

	bookmark = server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), bookmark)

	bookmarks := strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.NotContains(bookmarks, "No posts.")
	assert.Contains(bookmarks, "> Hello 1")
	assert.Contains(bookmarks, "> Hello 2")
}

func TestBookmark_Twice(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%201", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	bookmark := server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), bookmark)

	server.cfg.MinBookmarkInterval = 0

	bookmark = server.Handle("/users/bookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal("40 Error\r\n", bookmark)

	bookmarks := strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.NotContains(bookmarks, "No posts.")
	assert.Contains(bookmarks, "> Hello 1")

	unbookmark := server.Handle("/users/unbookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), unbookmark)

	unbookmark = server.Handle("/users/unbookmark/"+say[15:len(say)-2], server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", say[15:len(say)-2]), unbookmark)

	bookmarks = strings.Split(server.Handle("/users/bookmarks", server.Bob), "\n")
	assert.Contains(bookmarks, "No posts.")
	assert.NotContains(bookmarks, "> Hello 1")
}
