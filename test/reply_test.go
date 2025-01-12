/*
Copyright 2023 - 2025 Dima Krasner

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
	"github.com/dimkr/tootik/inbox"
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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Bob)
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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Bob)
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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Bob)
	assert.Contains(users, "Welcome Bob")

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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Bob)
	assert.NotContains(users, "Welcome Bob")

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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Bob)
	assert.NotContains(users, "Welcome Bob")

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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Bob)
	assert.Contains(users, "Welcome Bob")

	local := server.Handle("/local", nil)
	assert.NotContains(local, "Hello world")
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

	_, err := server.db.Exec("update outbox set inserted = inserted - 3600 where activity->>'$.type' = 'Create'")
	assert.NoError(err)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Welcome%%20me", id), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	view = server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello world")
	assert.Contains(view, "Welcome me")

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Bob)
	assert.Contains(users, "Welcome me")

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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
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

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
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

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.Contains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.Contains(users, "Hello Bob")
}

func TestReply_DMUnfollowed(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.Contains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.Contains(users, "Hello Bob")
}

func TestReply_DMUnfollowedBeforeFeedUpdate(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	users := server.Handle("/users", server.Alice)
	assert.NotContains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", id), server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "Hello @alice@localhost.localdomain:8443")
	assert.Contains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.Contains(users, "Hello Bob")
}

func TestReply_DMToAnotherUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	id := dm[15 : len(dm)-2]

	view := server.Handle("/users/view/"+id, server.Alice)
	assert.Contains(view, "Hello @alice@localhost.localdomain:8443")

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Bob", id), server.Carol)
	assert.Equal("40 Post not found\r\n", reply)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")
	assert.NotContains(users, "Hello Bob")
}

func TestReply_NoSuchPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	reply := server.Handle("/users/reply/x?Welcome%%20Bob", server.Alice)
	assert.Equal("40 Post not found\r\n", reply)
}
