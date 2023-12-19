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
	"time"
)

func TestInbox_NoPosts(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "No posts.")
}

func TestInbox_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	today := server.Handle("/users/inbox/today", nil)
	assert.Equal("30 /users\r\n", today)
}

func TestInbox_InvalidOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	today := server.Handle("/users/inbox/today?zz", server.Alice)
	assert.Equal("40 Invalid query\r\n", today)
}

func TestInbox_FutureDate(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	today := server.Handle("/users/inbox/"+time.Now().Add(time.Hour*24).Format(time.DateOnly), server.Alice)
	assert.Equal("30 /users/oops\r\n", today)
}

func TestInbox_InvalidDate(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	today := server.Handle("/users/inbox/9999-99-99", server.Alice)
	assert.Equal("40 Invalid date\r\n", today)
}

func TestInbox_PostToFollowersToday(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestInbox_PostToFollowersTodayBigOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today?123", server.Alice)
	assert.NotContains(today, "Hello world")
}

func TestInbox_PostToFollowersTodayByDate(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/"+time.Now().Format(time.DateOnly), server.Alice)
	assert.Contains(today, "Hello world")
}

func TestInbox_PostToFollowersYesterday(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	yesterday := server.Handle("/users/inbox/yesterday", server.Alice)
	assert.Contains(yesterday, "No posts.")
}

func TestInbox_MentionAndNoMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20%40alice%21", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	whisper = server.Handle("/users/whisper?Hello%20alice%21", server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	postWithMention := strings.Index(today, "Hello @alice!")
	postWithoutMention := strings.Index(today, "Hello alice!")
	assert.NotEqual(postWithMention, -1)
	assert.NotEqual(postWithoutMention, -1)
	assert.True(postWithMention < postWithoutMention)
}

func TestInbox_LeadingMentionAndNoMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	whisper := server.Handle("/users/whisper?%40alice%20Hello", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	whisper = server.Handle("/users/whisper?Hello%20alice%21", server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	postWithMention := strings.Index(today, "@alice Hello")
	postWithoutMention := strings.Index(today, "Hello alice!")
	assert.NotEqual(postWithMention, -1)
	assert.NotEqual(postWithoutMention, -1)
	assert.True(postWithMention < postWithoutMention)
}

func TestInbox_LeadingMentionAndCommaAndNoMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	whisper := server.Handle("/users/whisper?%40alice%2c%20Hello", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	whisper = server.Handle("/users/whisper?Hello%20alice%21", server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	postWithMention := strings.Index(today, "@alice, Hello")
	postWithoutMention := strings.Index(today, "Hello alice!")
	assert.NotEqual(postWithMention, -1)
	assert.NotEqual(postWithoutMention, -1)
	assert.True(postWithMention < postWithoutMention)
}

func TestInbox_NoMentionAndMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20alice%21", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	whisper = server.Handle("/users/whisper?Hello%20%40alice%21", server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	postWithMention := strings.Index(today, "Hello @alice!")
	postWithoutMention := strings.Index(today, "Hello alice!")
	assert.NotEqual(postWithMention, -1)
	assert.NotEqual(postWithoutMention, -1)
	assert.True(postWithMention < postWithoutMention)
}

func TestInbox_NoMentionAndMentionWithHost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20alice%21", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	whisper = server.Handle("/users/whisper?Hello%20%40alice%40localhost.localdomain%3a8443%21", server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	postWithMention := strings.Index(today, "Hello @alice@localhost.localdomain:8443!")
	postWithoutMention := strings.Index(today, "Hello alice!")
	assert.NotEqual(postWithMention, -1)
	assert.NotEqual(postWithoutMention, -1)
	assert.True(postWithMention < postWithoutMention)
}

func TestInbox_DMWithoutMentionAndMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20Alice", sha256.Sum256([]byte(server.Alice.ID))), server.Bob)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	whisper := server.Handle("/users/whisper?Hello%20%40alice%21", server.Carol)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	dmWithoutMention := strings.Index(today, "Hello Alice")
	postWithMention := strings.Index(today, "Hello @alice!")
	assert.NotEqual(dmWithoutMention, -1)
	assert.NotEqual(postWithMention, -1)
	assert.True(dmWithoutMention < postWithMention)
}

func TestInbox_MentionAndDMWithoutMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Bob.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Bob.ID))), follow)

	follow = server.Handle(fmt.Sprintf("/users/follow/%x", sha256.Sum256([]byte(server.Carol.ID))), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%x\r\n", sha256.Sum256([]byte(server.Carol.ID))), follow)

	whisper := server.Handle("/users/whisper?Hello%20%40alice%21", server.Bob)
	assert.Regexp("30 /users/view/[0-9a-f]{64}", whisper)

	dm := server.Handle(fmt.Sprintf("/users/dm/%x?Hello%%20Alice", sha256.Sum256([]byte(server.Alice.ID))), server.Carol)
	assert.Regexp("^30 /users/view/[0-9a-f]{64}\r\n$", dm)

	today := server.Handle("/users/inbox/today", server.Alice)
	dmWithoutMention := strings.Index(today, "Hello Alice")
	postWithMention := strings.Index(today, "Hello @alice!")
	assert.NotEqual(dmWithoutMention, -1)
	assert.NotEqual(postWithMention, -1)
	assert.True(dmWithoutMention < postWithMention)
}
