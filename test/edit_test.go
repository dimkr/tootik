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
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/dimkr/tootik/outbox"
	"github.com/stretchr/testify/assert"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestEdit_Throttling(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", hash), server.Bob)
	assert.Equal("40 Please try again later\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestEdit_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", hash), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello followers")

	edit = server.Handle(fmt.Sprintf("/users/edit/%s?Hello,%%20followers", hash), server.Bob)
	assert.Equal("40 Please try again later\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello followers")
}

func TestEdit_EmptyContent(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?", hash), server.Bob)
	assert.Equal("10 Post content\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestEdit_LongContent(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", hash), server.Bob)
	assert.Equal("40 Post is too long\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestEdit_InvalidEscapeSequence(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%zzworld", hash), server.Bob)
	assert.Equal("40 Bad input\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestEdit_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	edit := server.Handle("/users/edit/87428fc522803d31065e7bce3cf03fe475096631e5e07bbd7a0fde60c4cf25c7?Hello%20followers", server.Bob)
	assert.Equal("40 Error\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestEdit_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	hash := whisper[15 : len(whisper)-2]

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", hash), nil)
	assert.Equal("30 /users\r\n", edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestEdit_AddHashtag(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%23Hello%20world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hashtag := server.Handle("/users/hashtag/hello", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/users/hashtag/world", server.Bob)
	assert.NotContains(hashtag, server.Alice.PreferredUsername)

	hash := say[15 : len(say)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%23Hello%%20%%23world", hash), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	hashtag = server.Handle("/hashtag/hello", nil)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/hashtag/World", nil)
	assert.Contains(hashtag, server.Alice.PreferredUsername)
}

func TestEdit_RemoveHashtag(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%23Hello%20%23world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hashtag := server.Handle("/users/hashtag/hello", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/users/hashtag/world", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hash := say[15 : len(say)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%23Hello%%20world", hash), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	hashtag = server.Handle("/hashtag/hello", nil)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/hashtag/World", nil)
	assert.NotContains(hashtag, server.Alice.PreferredUsername)
}

func TestEdit_KeepHashtags(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%23Hello%20%23world", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hashtag := server.Handle("/users/hashtag/hello", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/users/hashtag/world", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hash := say[15 : len(say)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%23Hello%%20%%20%%23world", hash), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	hashtag = server.Handle("/hashtag/hello", nil)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/hashtag/World", nil)
	assert.Contains(hashtag, server.Alice.PreferredUsername)
}

func TestEdit_AddMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	lines := strings.Split(server.Handle("/users/view/"+hash, server.Alice), "\n")
	assert.Contains(lines, "> Hello world")
	assert.NotContains(lines, "> Hello @alice")
	assert.NotContains(lines, fmt.Sprintf("=> /users/outbox/%x alice", sha256.Sum256([]byte(server.Alice.ID))))

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20%%40alice", hash), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	lines = strings.Split(server.Handle("/users/view/"+hash, server.Alice), "\n")
	assert.NotContains(lines, "> Hello world")
	assert.Contains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%x alice", sha256.Sum256([]byte(server.Alice.ID))))
}

func TestEdit_RemoveMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	say := server.Handle("/users/say?Hello%20%40alice", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	lines := strings.Split(server.Handle("/users/view/"+hash, server.Alice), "\n")
	assert.NotContains(lines, "> Hello world")
	assert.Contains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%x alice", sha256.Sum256([]byte(server.Alice.ID))))

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20world", hash), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	lines = strings.Split(server.Handle("/users/view/"+hash, server.Alice), "\n")
	assert.Contains(lines, "> Hello world")
	assert.NotContains(lines, "> Hello @alice")
	assert.NotContains(lines, fmt.Sprintf("=> /users/outbox/%x alice", sha256.Sum256([]byte(server.Alice.ID))))
}

func TestEdit_KeepMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	say := server.Handle("/users/say?Hello%20%40alice", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	lines := strings.Split(server.Handle("/users/view/"+hash, server.Alice), "\n")
	assert.NotContains(lines, "> Hello  @alice")
	assert.Contains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%x alice", sha256.Sum256([]byte(server.Alice.ID))))

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20%%20%%40alice", hash), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	lines = strings.Split(server.Handle("/users/view/"+hash, server.Alice), "\n")
	assert.Contains(lines, "> Hello  @alice")
	assert.NotContains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%x alice", sha256.Sum256([]byte(server.Alice.ID))))
}

func TestEdit_AddGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/people",
		"4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20people%40other.localdomain", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello people")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20people%%20in%%20%%40people%%40other.localdomain", hash), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people")
}

func TestEdit_RemoveGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/people",
		"4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20people%%20in%%20people%%40other.localdomain", hash), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello people")
}

func TestEdit_ChangeGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/adults",
		"91511969848852e647a2afb90d269fae0bd5f5e1edd92dc07348fd5a25d5da86",
		`{"type":"Group","preferredUsername":"adults"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/people",
		"4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40adults%40other.localdomain", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello people")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20people%%20in%%20%%40people%%40other.localdomain", hash), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people")
}

func TestEdit_AddReplyGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/people",
		"4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20there", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello there")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), replyHash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20people%%20in%%20%%40people%%40other.localdomain", replyHash), server.Carol)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", replyHash), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people") // reply group is determined by cc
}

func TestEdit_ChangeReplyGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/adults",
		"91511969848852e647a2afb90d269fae0bd5f5e1edd92dc07348fd5a25d5da86",
		`{"type":"Group","preferredUsername":"adults"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/people",
		"4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20there", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello there")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), replyHash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20adults%%20in%%20%%40adults%%40other.localdomain", replyHash), server.Carol)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", replyHash), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello adults") // reply group is determined by cc
}

func TestEdit_RemoveReplyGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/adults",
		"91511969848852e647a2afb90d269fae0bd5f5e1edd92dc07348fd5a25d5da86",
		`{"type":"Group","preferredUsername":"adults"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, hash, actor) values(?,?,?)`,
		"https://other.localdomain/group/people",
		"4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20adults%%20in%%20%%40adults%%40other.localdomain", hash), server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", reply)

	replyHash := reply[15 : len(reply)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello adults")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), replyHash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20adults", replyHash), server.Carol)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", replyHash), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello adults") // reply group is determined by first group in cc
}

func TestEdit_PollAddOption(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", say[15:len(say)-2]), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.NotContains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%5bPOLL%%20So%%2c%%20polls%%20on%%20Station%%20are%%20pretty%%20cool%%2c%%20right%%3f%%5d%%20Nope%%20%%7c%%20Hell%%20yeah%%21%%20%%7c%%20I%%20couldn%%27t%%20care%%20less", hash), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?I%%20couldn%%27t%%20care%%20less", say[15:len(say)-2]), server.Carol)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view = server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.Contains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
}

func TestEdit_RemoveQuestion(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?%5bPOLL%20So%2c%20polls%20on%20Station%20are%20pretty%20cool%2c%20right%3f%5d%20Nope%20%7c%20Hell%20yeah%21%20", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	hash := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", say[15:len(say)-2]), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view := server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.NotContains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where hash = ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), hash)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?This%%20is%%20not%%20a%%20poll", hash), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", hash), edit)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view = server.Handle("/users/view/"+hash, server.Bob)
	assert.Contains(view, "This is not a poll")
	assert.NotContains(view, "Vote")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
}
