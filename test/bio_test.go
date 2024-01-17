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
	"time"
)

func TestBio_Throttled(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	summary := server.Handle("/users/bio?Hello%20world", server.Alice)
	assert.Equal("40 Please try again later\r\n", summary)
}

func TestBio_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/bio?Hello%20world", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), summary)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(strings.Split(outbox, "\n"), "> Hello world")
}

func TestBio_TooLong(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/bio?aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", server.Alice)
	assert.Equal("40 Summary is too long\r\n", summary)
}

func TestBio_MultiLine(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/bio?Hello%0Aworld", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), summary)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob), "\n")
	assert.Contains(outbox, "> Hello")
	assert.Contains(outbox, "> world")
}

func TestBio_MultiLineWithLink(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/bio?Hi%21%0A%0AI%27m%20a%20friend%20of%20https%3a%2f%2flocalhost.localdomain%3a8443%2fuser%2fbob", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), summary)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob), "\n")
	assert.Contains(outbox, "> Hi!")
	assert.Contains(outbox, "> I'm a friend of https://localhost.localdomain:8443/user/bob")
	assert.Contains(outbox, "=> https://localhost.localdomain:8443/user/bob https://localhost.localdomain:8443/user/bob")
}
