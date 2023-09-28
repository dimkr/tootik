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

func TestResolve_LocalUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?alice%40localhost", server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), resolve)
}

func TestResolve_LocalUserByNameOnly(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?alice", server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), resolve)
}

func TestResolve_NoSuchLocalUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?troll%40localhost", server.Bob)
	assert.Equal("40 Failed to resolve troll@localhost\r\n", resolve)
}

func TestResolve_NoSuchLocalUserByNameOnly(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?troll", server.Bob)
	assert.Equal("40 Failed to resolve troll@localhost\r\n", resolve)
}

func TestResolve_NoSuchFederatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?troll%400.0.0.0", server.Bob)
	assert.Equal("40 Failed to resolve troll@0.0.0.0\r\n", resolve)
}

func TestResolve_NoInput(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?", server.Bob)
	assert.Equal("10 User name (name or name@domain)\r\n", resolve)
}

func TestResolve_InvalidEscapeSequence(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?troll%zzlocalhost ", server.Bob)
	assert.Equal("40 Bad input\r\n", resolve)
}

func TestResolve_InvalidInputFormat(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?troll%40localhost%400.0.0.0", server.Bob)
	assert.Equal("40 Bad input\r\n", resolve)
}

func TestResolve_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	resolve := server.Handle("/users/resolve?alice", nil)
	assert.Equal("30 /users\r\n", resolve)
}
