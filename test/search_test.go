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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSearch_Happyflow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	search := server.Handle("/users/search?world", server.Bob)
	assert.Equal("30 /users/hashtag/world\r\n", search)
}

func TestSearch_LeadingHash(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	search := server.Handle("/users/search?%23world", server.Bob)
	assert.Equal("30 /users/hashtag/world\r\n", search)
}

func TestSearch_LeadingHashUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	search := server.Handle("/search?%23world", nil)
	assert.Equal("30 /hashtag/world\r\n", search)
}

func TestSearch_NoInput(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	search := server.Handle("/users/search?", server.Bob)
	assert.Equal("10 Hashtag\r\n", search)
}

func TestSearch_EmptyInput(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	search := server.Handle("/users/search?", server.Bob)
	assert.Equal("10 Hashtag\r\n", search)
}

func TestSearch_InvalidEscapeSequence(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	search := server.Handle("/users/search?%zzworld", server.Bob)
	assert.Equal("40 Bad input\r\n", search)
}

func TestSearch_UnathenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	search := server.Handle("/search?world", nil)
	assert.Equal("30 /hashtag/world\r\n", search)
}
