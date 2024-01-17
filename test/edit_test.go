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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", id), server.Bob)
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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Nothing to see! Are you following anyone?")
	assert.Contains(users, "1 post")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello followers")

	edit = server.Handle(fmt.Sprintf("/users/edit/%s?Hello,%%20followers", id), server.Bob)
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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?", id), server.Bob)
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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", id), server.Bob)
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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%zzworld", id), server.Bob)
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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, whisper)

	edit := server.Handle("/users/edit/x?Hello%20followers", server.Bob)
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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, whisper)

	id := whisper[15 : len(whisper)-2]

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20followers", id), nil)
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
	assert.Regexp(`^30 /users/view/\S+`, say)

	hashtag := server.Handle("/users/hashtag/hello", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/users/hashtag/world", server.Bob)
	assert.NotContains(hashtag, server.Alice.PreferredUsername)

	id := say[15 : len(say)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%23Hello%%20%%23world", id), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

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
	assert.Regexp(`^30 /users/view/\S+`, say)

	hashtag := server.Handle("/users/hashtag/hello", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/users/hashtag/world", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	id := say[15 : len(say)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%23Hello%%20world", id), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

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
	assert.Regexp(`^30 /users/view/\S+`, say)

	hashtag := server.Handle("/users/hashtag/hello", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	hashtag = server.Handle("/users/hashtag/world", server.Bob)
	assert.Contains(hashtag, server.Alice.PreferredUsername)

	id := say[15 : len(say)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%23Hello%%20%%20%%23world", id), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

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
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	lines := strings.Split(server.Handle("/users/view/"+id, server.Alice), "\n")
	assert.Contains(lines, "> Hello world")
	assert.NotContains(lines, "> Hello @alice")
	assert.NotContains(lines, fmt.Sprintf("=> /users/outbox/%s alice", strings.TrimPrefix(server.Alice.ID, "https://")))

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20%%40alice", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	lines = strings.Split(server.Handle("/users/view/"+id, server.Alice), "\n")
	assert.NotContains(lines, "> Hello world")
	assert.Contains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%s alice", strings.TrimPrefix(server.Alice.ID, "https://")))
}

func TestEdit_RemoveMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	say := server.Handle("/users/say?Hello%20%40alice", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	lines := strings.Split(server.Handle("/users/view/"+id, server.Alice), "\n")
	assert.NotContains(lines, "> Hello world")
	assert.Contains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%s alice", strings.TrimPrefix(server.Alice.ID, "https://")))

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20world", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	lines = strings.Split(server.Handle("/users/view/"+id, server.Alice), "\n")
	assert.Contains(lines, "> Hello world")
	assert.NotContains(lines, "> Hello @alice")
	assert.NotContains(lines, fmt.Sprintf("=> /users/outbox/%s alice", strings.TrimPrefix(server.Alice.ID, "https://")))
}

func TestEdit_KeepMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Nothing to see! Are you following anyone?")
	assert.NotContains(users, "1 post")

	say := server.Handle("/users/say?Hello%20%40alice", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	lines := strings.Split(server.Handle("/users/view/"+id, server.Alice), "\n")
	assert.NotContains(lines, "> Hello  @alice")
	assert.Contains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%s alice", strings.TrimPrefix(server.Alice.ID, "https://")))

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20%%20%%40alice", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	lines = strings.Split(server.Handle("/users/view/"+id, server.Alice), "\n")
	assert.Contains(lines, "> Hello  @alice")
	assert.NotContains(lines, "> Hello @alice")
	assert.Contains(lines, fmt.Sprintf("=> /users/outbox/%s alice", strings.TrimPrefix(server.Alice.ID, "https://")))
}

func TestEdit_AddGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20people%40other.localdomain", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n`, say)

	id := say[15 : len(say)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello people")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20people%%20in%%20%%40people%%40other.localdomain", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people")
}

func TestEdit_RemoveGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20people%%20in%%20people%%40other.localdomain", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello people")
}

func TestEdit_ChangeGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/adults",
		`{"type":"Group","preferredUsername":"adults"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40adults%40other.localdomain", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello people")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?Hello%%20people%%20in%%20%%40people%%40other.localdomain", id), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people")
}

func TestEdit_AddReplyGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20there", id), server.Carol)
	assert.Regexp(`^30 /users/view/\S+`, reply)

	replyHash := reply[15 : len(reply)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello there")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), replyHash)
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
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/adults",
		`{"type":"Group","preferredUsername":"adults"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20there", id), server.Carol)
	assert.Regexp(`^30 /users/view/\S+`, reply)

	replyHash := reply[15 : len(reply)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello there")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), replyHash)
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
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/adults",
		`{"type":"Group","preferredUsername":"adults"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp(`^30 /users/view/\S+`, say)

	id := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20adults%%20in%%20%%40adults%%40other.localdomain", id), server.Carol)
	assert.Regexp(`^30 /users/view/\S+`, reply)

	replyHash := reply[15 : len(reply)-2]

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello adults")

	_, err = server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), replyHash)
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
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", say[15:len(say)-2]), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.NotContains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?%%5bPOLL%%20So%%2c%%20polls%%20on%%20Station%%20are%%20pretty%%20cool%%2c%%20right%%3f%%5d%%20Nope%%20%%7c%%20Hell%%20yeah%%21%%20%%7c%%20I%%20couldn%%27t%%20care%%20less", id), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	reply = server.Handle(fmt.Sprintf("/users/reply/%s?I%%20couldn%%27t%%20care%%20less", say[15:len(say)-2]), server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view = server.Handle("/users/view/"+id, server.Bob)
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
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hell%%20yeah%%21", say[15:len(say)-2]), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "So, polls on Station are pretty cool, right?")
	assert.Contains(view, "Vote Nope")
	assert.Contains(view, "Vote Hell yeah!")
	assert.NotContains(view, "Vote I couldn't care less")
	assert.Contains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Handle(fmt.Sprintf("/users/edit/%s?This%%20is%%20not%%20a%%20poll", id), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), edit)

	assert.NoError(outbox.UpdatePollResults(context.Background(), domain, slog.Default(), server.db))

	view = server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "This is not a poll")
	assert.NotContains(view, "Vote")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ Hell yeah!")
	assert.NotContains(strings.Split(view, "\n"), "0          I couldn't care less")
	assert.NotContains(strings.Split(view, "\n"), "1 ████████ I couldn't care less")
}
