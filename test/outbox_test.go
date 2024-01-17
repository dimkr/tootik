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

func TestOutbox_NonExistingUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	outbox := server.Handle("/users/outbox/x", server.Bob)
	assert.Equal("40 User not found\r\n", outbox)
}

func TestOutbox_InvalidOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%s?abc", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Equal("40 Invalid query\r\n", outbox)
}

func TestOutbox_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	outbox := server.Handle("/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), nil)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostSelf(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Alice)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersUnauthentictedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	outbox := server.Handle("/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), nil)
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersSelf(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Alice)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_DM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Hello bob")
}

func TestOutbox_DMSelf(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Alice)
	assert.Contains(outbox, "Hello bob")
}

func TestOutbox_DMNotRecipient(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol)
	assert.NotContains(outbox, "Hello bob")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), nil)
	assert.NotContains(outbox, "Hello bob")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PublicPostInGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, say)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello people in @people@other.localdomain")
}

func TestOutbox_PublicPostInGroupUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, say)

	outbox := server.Handle("/outbox/other.localdomain/group/people", nil)
	assert.Contains(outbox, "Hello people in @people@other.localdomain")
}

func TestOutbox_PostToFollowersInGroup(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello people in @people@other.localdomain")
}

func TestOutbox_PostToFollowersInGroupNotFollowingGroup(t *testing.T) {
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

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PostToFollowersInGroupNotAccepted(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PostToFollowersInGroupFollowingAuthor(t *testing.T) {
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

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PostToFollowersInGroupUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/outbox/other.localdomain/group/people", nil)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_DMInGroup(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupNotFollowingGroup(t *testing.T) {
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

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello bob from @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_DMInGroupAnotherUser(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Carol)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Carol)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupSelf(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupSelfGroupUnfollowed(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	unfollow := server.Handle("/users/unfollow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", unfollow)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_DMInGroupSelfRecipientUnfollowed(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), unfollow)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupAnauthenticatedUser(t *testing.T) {
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

	follow = server.Handle("/users/follow/other.localdomain/group/people", server.Bob)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", strings.TrimPrefix(server.Bob.ID, "https://")), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n`, whisper)

	outbox := server.Handle("/outbox/other.localdomain/group/people", nil)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}
