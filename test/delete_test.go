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

func TestDelete_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")

	delete := server.Handle("/users/delete/"+hash, server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), delete)

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Equal(t, view, "40 Post not found\r\n")
}

func TestDelete_NotAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")

	delete := server.Handle("/users/delete/"+hash, server.Bob)
	assert.Equal(t, delete, "40 Error\r\n")

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
}

func TestDelete_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	delete := server.Handle("/users/delete/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7", server.Alice)
	assert.Equal(t, delete, "40 Error\r\n")
}

func TestDelete_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(t, view, "Hello world")

	delete := server.Handle("/users/delete/"+hash, nil)
	assert.Equal(t, delete, "30 /users\r\n")

	view = server.Handle("/users/view/"+hash, server.Alice)
	assert.Contains(t, view, "Hello world")
}
