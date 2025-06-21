/*
Copyright 2024, 2025 Dima Krasner

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
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBio_Throttled(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	bio := server.Handle("/users/bio/set?Hello%20world", server.Alice)
	assert.Regexp(`^40 Please wait for \S+\r\n$`, bio)
}

func TestBio_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	bio := server.Handle("/users/bio/set?Hello%20world", server.Alice)
	assert.Equal("30 /users/bio\r\n", bio)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(strings.Split(outbox, "\n"), "> Hello world")
}

func TestBio_TooLong(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	bio := server.Handle("/users/bio/set?aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", server.Alice)
	assert.Equal("40 Bio is too long\r\n", bio)
}

func TestBio_MultiLine(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	bio := server.Handle("/users/bio/set?Hello%0Aworld", server.Alice)
	assert.Equal("30 /users/bio\r\n", bio)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob), "\n")
	assert.Contains(outbox, "> Hello")
	assert.Contains(outbox, "> world")
}

func TestBio_MultiLineWithLink(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	bio := server.Handle("/users/bio/set?Hi%21%0A%0AI%27m%20a%20friend%20of%20https%3a%2f%2flocalhost.localdomain%3a8443%2fuser%2fbob", server.Alice)
	assert.Equal("30 /users/bio\r\n", bio)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob), "\n")
	assert.Contains(outbox, "> Hi!")
	assert.Contains(outbox, "> I'm a friend of https://localhost.localdomain:8443/user/bob")
	assert.Contains(outbox, "=> https://localhost.localdomain:8443/user/bob https://localhost.localdomain:8443/user/bob")
}
