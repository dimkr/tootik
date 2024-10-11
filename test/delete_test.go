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

func TestDelete_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")

	delete := server.Handle("/users/delete/"+id, server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), delete)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Equal(view, "40 Post not found\r\n")
}

func TestDelete_NotAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")

	delete := server.Handle("/users/delete/"+id, server.Bob)
	assert.Equal(delete, "40 Error\r\n")

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
}

func TestDelete_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	delete := server.Handle("/users/delete/x", server.Alice)
	assert.Equal(delete, "40 Error\r\n")
}

func TestDelete_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")

	delete := server.Handle("/users/delete/"+id, nil)
	assert.Equal(delete, "30 /users\r\n")

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
}

func TestDelete_WithReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`30 /users/view/\S+\r\n$`, say)

	postID := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Alice", postID), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	replyID := reply[15 : len(reply)-2]

	delete := server.Handle("/users/delete/"+replyID, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), delete)

	view := server.Handle("/users/view/"+replyID, server.Alice)
	assert.Equal(view, "40 Post not found\r\n")

	delete = server.Handle("/users/delete/"+postID, server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), delete)

	view = server.Handle("/users/view/"+postID, server.Alice)
	assert.Equal(view, "40 Post not found\r\n")
}

func TestDelete_WithReplyPostDeletedFirst(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`30 /users/view/\S+\r\n$`, say)

	postID := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Alice", postID), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	replyID := reply[15 : len(reply)-2]

	delete := server.Handle("/users/delete/"+postID, server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), delete)

	view := server.Handle("/users/view/"+postID, server.Alice)
	assert.Equal(view, "40 Post not found\r\n")

	delete = server.Handle("/users/delete/"+replyID, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), delete)

	view = server.Handle("/users/view/"+replyID, server.Alice)
	assert.Equal(view, "40 Post not found\r\n")
}
