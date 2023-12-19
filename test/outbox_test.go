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
	"strings"
	"testing"
)

func TestOutbox_NonExistingUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	outbox := server.Handle("/users/outbox/1393ac075483a094823f4b88bc18accca757c2f9e68ca6bc6aa14fc841a292e4", server.Bob)
	assert.Equal("40 User not found\r\n", outbox)
}

func TestOutbox_InvalidOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x?abc", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal("40 Invalid query\r\n", outbox)
}

func TestOutbox_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	outbox := server.Handle(fmt.Sprintf("/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), nil)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostSelf(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", say)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Alice)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersUnauthentictedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	outbox := server.Handle(fmt.Sprintf("/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), nil)
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersSelf(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", whisper)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Alice)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_DM(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Contains(outbox, "Hello bob")
}

func TestOutbox_DMSelf(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Alice)
	assert.Contains(outbox, "Hello bob")
}

func TestOutbox_DMNotRecipient(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	outbox := server.Handle(fmt.Sprintf("/users/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Carol)
	assert.NotContains(outbox, "Hello bob")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	outbox := server.Handle(fmt.Sprintf("/outbox/%x", sha256.Sum256([]byte(server.Alice.ID))), nil)
	assert.NotContains(outbox, "Hello bob")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PublicPostInGroup(t *testing.T) {
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

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain%3a8443", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Contains(outbox, "Hello people in @people@other.localdomain")
}

func TestOutbox_PublicPostInGroupUnauthenticatedUser(t *testing.T) {
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

	say := server.Handle("/users/say?Hello%20people%20in%20%40people%40other.localdomain%3a8443", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", say)

	outbox := server.Handle("/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", nil)
	assert.Contains(outbox, "Hello people in @people@other.localdomain")
}

func TestOutbox_PostToFollowersInGroup(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain%3a8443", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Contains(outbox, "Hello people in @people@other.localdomain")
}

func TestOutbox_PostToFollowersInGroupNotFollowingGroup(t *testing.T) {
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

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain%3a8443", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PostToFollowersInGroupNotAccepted(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain%3a8443", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PostToFollowersInGroupFollowingAuthor(t *testing.T) {
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

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain%3a8443", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PostToFollowersInGroupUnauthenticatedUser(t *testing.T) {
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

	whisper := server.Handle("/users/whisper?Hello%20people%20in%20%40people%40other.localdomain%3a8443", server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", nil)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_DMInGroup(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupNotFollowingGroup(t *testing.T) {
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

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.NotContains(outbox, "Hello bob from @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_DMInGroupAnotherUser(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Carol)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Carol)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupSelf(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupSelfGroupUnfollowed(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	unfollow := server.Handle("/users/unfollow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", unfollow)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_DMInGroupSelfRecipientUnfollowed(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	unfollow := server.Handle(fmt.Sprintf("/users/unfollow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), unfollow)

	outbox := server.Handle("/users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Alice)
	assert.Contains(outbox, "Hello bob from @people@other.localdomain")
}

func TestOutbox_DMInGroupAnauthenticatedUser(t *testing.T) {
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

	follow = server.Handle("/users/follow/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", server.Bob)
	assert.Equal("30 /users/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Alice.ID))), follow)

	whisper := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20bob%%20from%%20%%40people%%40other.localdomain", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	outbox := server.Handle("/outbox/4eeaa25305ef85dec1dc646e02f54fc1702f594d5bc0c8b9b1c41595a16ea70f", nil)
	assert.NotContains(outbox, "Hello people in @people@other.localdomain")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}
