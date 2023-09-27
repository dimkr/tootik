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
	"time"
)

func TestEdit_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(t, users, "Nothing to see! Are you following anyone?")
	assert.NotContains(t, users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", hash), server.Bob)
	assert.Equal(t, "40 Please try again later\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello world")
}

func TestEdit_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(t, users, "Nothing to see! Are you following anyone?")
	assert.NotContains(t, users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", hash), server.Bob)
	assert.Equal(t, fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello followers")

	edit = server.Handle(fmt.Sprintf("/users/edit/%s?Hello,%%20followers", hash), server.Bob)
	assert.Equal(t, "40 Please try again later\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello followers")
}

func TestEdit_EmptyContent(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(t, users, "Nothing to see! Are you following anyone?")
	assert.NotContains(t, users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?", hash), server.Bob)
	assert.Equal(t, "10 Post content\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello world")
}

func TestEdit_LongContent(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(t, users, "Nothing to see! Are you following anyone?")
	assert.NotContains(t, users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", hash), server.Bob)
	assert.Equal(t, "40 Post is too long\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello world")
}

func TestEdit_InvalidEscapeSequence(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(t, users, "Nothing to see! Are you following anyone?")
	assert.NotContains(t, users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%zzworld", hash), server.Bob)
	assert.Equal(t, "40 Bad input\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello world")
}

func TestEdit_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(t, users, "Nothing to see! Are you following anyone?")
	assert.NotContains(t, users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	edit := server.Handle("/users/edit/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7?Hello%20followers", server.Bob)
	assert.Equal(t, "40 Error\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello world")
}

func TestEdit_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(t, fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(t, users, "Nothing to see! Are you following anyone?")
	assert.NotContains(t, users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(t, "30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", hash), nil)
	assert.Equal(t, "30 /users\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(t, users, "Nothing to see! Are you following anyone?")
	assert.Contains(t, users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(t, today, "Hello world")
}
