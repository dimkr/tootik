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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDM_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443")

	view = server.Handle("/users/view/"+id, server.Carol)
	assert.Equal(view, "40 Post not found\r\n")

	view = server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443")
}

func TestDM_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	id := dm[15 : len(dm)-2]

	view := server.Handle("/view/"+id, nil)
	assert.Equal(view, "40 Post not found\r\n")
}

func TestDM_Loopback(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle("/users/dm?Hello%20%40bob%40localhost.localdomain%3a8443", server.Bob)
	assert.Equal("40 Post audience is empty\r\n", dm)
}

func TestDM_TwoMentions(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443%20and%20%40carol%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @carol@localhost.localdomain:8443")

	view = server.Handle("/users/view/"+id, server.Carol)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @carol@localhost.localdomain:8443")

	view = server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @carol@localhost.localdomain:8443")
}

func TestDM_TwoMentionsOneLoopback(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443%20and%20%40bob%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @bob@localhost.localdomain:8443")

	view = server.Handle("/users/view/"+id, server.Carol)
	assert.Equal(view, "40 Post not found\r\n")

	view = server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @bob@localhost.localdomain:8443")
}

func TestDM_TooManyRecipients(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.MaxRecipients = 1

	assert := assert.New(t)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443%20and%20%40carol%40localhost.localdomain%3a8443", server.Bob)
	assert.Equal("40 Too many recipients\r\n", dm)
}

func TestDM_MaxRecipients(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	server.cfg.MaxRecipients = 2

	assert := assert.New(t)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443%20and%20%40carol%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @carol@localhost.localdomain:8443")

	view = server.Handle("/users/view/"+id, server.Carol)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @carol@localhost.localdomain:8443")

	view = server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443 and @carol@localhost.localdomain:8443")
}
