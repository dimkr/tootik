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

func TestReply_AuthorNotFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.Contains(local, "Hello world")
	assert.Contains(local, "Welcome Bob")
}

func TestReply_AuthorFollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.Contains(local, "Hello world")
	assert.Contains(local, "Welcome Bob")
}

func TestReply_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	today := server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(today, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(local, "Hello world")
	assert.NotContains(local, "Welcome Bob")
}

func TestReply_PostToFollowersNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Alice)
	assert.Equal("40 Post not found\r\n", reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Equal("40 Post not found\r\n", view)

	today := server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(local, "Hello world")
	assert.NotContains(local, "Welcome Bob")
}

func TestReply_PostToFollowersUnfollowedBeforeReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Alice)
	assert.Equal("40 Post not found\r\n", reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.NotContains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	today := server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(local, "Hello world")
	assert.NotContains(local, "Welcome Bob")
}

func TestReply_PostToFollowersUnfollowedAfterReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Equal("40 Post not found\r\n", view)

	today := server.Handle("/users/inbox/today", server.Bob)
	assert.Contains(today, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(local, "Hello world")
	assert.NotContains(local, "Welcome Bob")
}

func TestReply_PostToFollowersInFollowedGroup(t *testing.T) {
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

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello people in @people@other.localdomain")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello people in @people@other.localdomain")
	assert.Contains(view, "Welcome Bob")

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello people in @people@other.localdomain")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Hello people in @people@other.localdomain")
	assert.Contains(today, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(local, "Hello people in @people@other.localdomain")
	assert.NotContains(local, "Welcome Bob")
}

func TestReply_SelfReply(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	_, err := server.db.Exec("update outbox set inserted = inserted - 3600 where activity->>'type' = 'Create'")
	assert.NoError(err)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20me", id), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome me")

	today := server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Welcome me")

	local := server.Handle("/local", nil)
	assert.NotContains(local, "Hello world")
	assert.NotContains(local, "Welcome me")
}

func TestReply_ReplyToPublicPostByFollowedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(users, "Hello world")
	assert.NotContains(users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.Contains(local, "Hello world")
	assert.Contains(local, "Welcome Bob")
}

func TestReply_ReplyToPublicPostByNotFollowedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]

	view := server.Handle("/users/view/"+id, server.Bob)
	assert.Contains(view, "Hello world")
	assert.NotContains(view, "Welcome Bob")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20Bob", id), server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome Bob")

	users := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(users, "Hello world")
	assert.NotContains(users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.Contains(local, "Hello world")
	assert.Contains(local, "Welcome Bob")
}

func TestReply_DM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello Alice")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Hello Alice")
	assert.Contains(today, "Hello Bob")
}

func TestReply_DMUnfollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello Alice")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Hello Alice")
	assert.Contains(today, "Hello Bob")
}

func TestReply_DMToAnotherUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello Alice")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", id), server.Carol)
	assert.Equal("40 Post not found\r\n", reply)

	today = server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")

	today = server.Handle("/users/inbox/today", server.Bob)
	assert.NotContains(today, "Hello Alice")
	assert.NotContains(today, "Hello Bob")
}

func TestReply_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	reply := server.Handle("/users/reply/x?Welcome%%20Bob", server.Alice)
	assert.Equal("40 Post not found\r\n", reply)
}
