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
	"crypto/sha256"
	"fmt"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

func TestName_Throttled(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	summary := server.Handle("/users/name?Jane%20Doe", server.Alice)
	assert.Equal("40 Please try again later\r\n", summary)
}

func TestName_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/name?Jane%20Doe", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), summary)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(strings.Split(outbox, "\n"), "# 😈 Jane Doe (alice@localhost.localdomain:8443)")
}

func TestName_TooLong(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/name?aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", server.Alice)
	assert.Equal("40 Display name is too long\r\n", summary)
}

func TestName_MultiLine(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)

	summary := server.Handle("/users/name?Jane%0A%0A%0A%0ADoe", server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), summary)

	outbox := strings.Split(server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob), "\n")
	assert.Contains(outbox, "# 😈 Jane Doe (alice@localhost.localdomain:8443)")
}
