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

func TestDM_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp(t, "^30 /users/outbox/[0-9a-f]{64}\r\n$", follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20Alice", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Regexp(t, "^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	view := server.Handle(dm[3:len(dm)-2], server.Alice)
	assert.Contains(t, view, "Hello Alice")
}

func TestDM_Loopback(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20world", sha256.Sum256([]byte(server.Alice.ID))), server.Alice)
	assert.Equal(t, "40 Error\r\n", dm)
}

func TestDM_NotFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20world", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(t, "40 Error\r\n", dm)
}

func TestDM_NoSuchUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	dm := server.Handle("/users/dm/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7?Hello%20world", server.Bob)
	assert.Equal(t, "40 User does not exist\r\n", dm)
}
