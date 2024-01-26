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
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/inbox"
	"github.com/stretchr/testify/assert"
	"log/slog"
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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestInbox_PostToFollowersTodayBigOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	today := server.Handle("/users/inbox/today?123", server.Alice)
	assert.NotContains(today, "Hello world")
}

func TestInbox_PostToFollowersTodayByDate(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	today := server.Handle("/users/inbox/"+time.Now().Format(time.DateOnly), server.Alice)
	assert.Contains(today, "Hello world")
}

func TestInbox_PostToFollowersYesterday(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	yesterday := server.Handle("/users/inbox/yesterday", server.Alice)
	assert.Contains(yesterday, "No posts.")
}

func TestInbox_MentionAndNoMention(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20%40alice%21", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	whisper = server.Handle("/users/whisper?Hello%20alice%21", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?%40alice%20Hello", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	whisper = server.Handle("/users/whisper?Hello%20alice%21", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?%40alice%2c%20Hello", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	whisper = server.Handle("/users/whisper?Hello%20alice%21", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20alice%21", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	whisper = server.Handle("/users/whisper?Hello%20%40alice%21", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20alice%21", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	whisper = server.Handle("/users/whisper?Hello%20%40alice%40localhost.localdomain%3a8443%21", server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	whisper := server.Handle("/users/whisper?Hello%20%40alice%21", server.Carol)
	assert.Regexp(`30 /users/view/\S+\r\n`, whisper)

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

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Carol.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Carol.ID, "https://")), follow)

	whisper := server.Handle("/users/whisper?Hello%20%40alice%21", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	dm := server.Handle(fmt.Sprintf("/users/dm/%s?Hello%%20Alice", strings.TrimPrefix(server.Alice.ID, "https://")), server.Carol)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	today := server.Handle("/users/inbox/today", server.Alice)
	dmWithoutMention := strings.Index(today, "Hello Alice")
	postWithMention := strings.Index(today, "Hello @alice!")
	assert.NotEqual(dmWithoutMention, -1)
	assert.NotEqual(postWithMention, -1)
	assert.True(dmWithoutMention < postWithMention)
}

func TestInbox_PublicPostShared(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/erin",
		`{"id":"https://127.0.0.1/user/erin","type":"Person","preferredUsername":"erin","followers":"https://127.0.0.1/followers/erin"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/127.0.0.1/user/erin", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/erin\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/erin",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Announce","actor":"https://127.0.0.1/user/erin","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/erin"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:   domain,
		Config:   server.cfg,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg),
		Actor:    server.Nobody,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.Contains(today, "Hello world")
}

func TestInbox_PublicPostSharedNotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"id":"https://127.0.0.1/user/dan","type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/erin",
		`{"id":"https://127.0.0.1/user/erin","type":"Person","preferredUsername":"erin","followers":"https://127.0.0.1/followers/erin"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/127.0.0.1/user/erin", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/erin\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity) values(?,?)`,
		"https://127.0.0.1/user/erin",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Announce","actor":"https://127.0.0.1/user/erin","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/erin"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:   domain,
		Config:   server.cfg,
		Log:      slog.Default(),
		DB:       server.db,
		Resolver: fed.NewResolver(nil, domain, server.cfg),
		Actor:    server.Nobody,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	unfollow := server.Handle("/users/unfollow/127.0.0.1/user/erin", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/erin\r\n", unfollow)

	today := server.Handle("/users/inbox/today", server.Alice)
	assert.NotContains(today, "Hello world")
}
