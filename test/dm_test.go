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

func TestDM_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	view := server.Handle(dm[3:len(dm)-2], server.Alice)
	assert.Contains(view, "Hello Alice")
}

func TestDM_Loopback(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20world", strings.TrimPrefix(server.Alice.ID, "https://")), server.Alice)
	assert.Equal("40 Error\r\n", dm)
}

func TestDM_NotFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20world", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Equal("40 Error\r\n", dm)
}

func TestDM_NoSuchUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle("/users/dm/x?Hello%20world", server.Bob)
	assert.Equal("40 User does not exist\r\n", dm)
}
