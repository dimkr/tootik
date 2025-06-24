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

func TestName_Throttled(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	summary := server.Handle("/users/name/set?Jane%20Doe", server.Alice)
	assert.Regexp(`^40 Please wait for \S+\r\n$`, summary)
}

func TestName_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/name/set?Jane%20Doe", server.Alice)
	assert.Equal("30 /users/name\r\n", summary)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(strings.Split(outbox, "\n"), "# 😈 Jane Doe (alice@localhost.localdomain:8443)")
}

func TestName_TooLong(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/name/set?aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", server.Alice)
	assert.Equal("40 Display name is too long\r\n", summary)
}

func TestName_MultiLine(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/name/set?Jane%0A%0A%0A%0ADoe", server.Alice)
	assert.Equal("30 /users/name\r\n", summary)

	outbox := strings.Split(server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob), "\n")
	assert.Contains(outbox, "# 😈 Jane Doe (alice@localhost.localdomain:8443)")
}
