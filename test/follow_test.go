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
	"github.com/dimkr/tootik/inbox"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestFollow_PostToFollowers(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello world")
}

func TestFollow_PostToFollowersBeforeFollow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	whisper := server.Handle("/users/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello world")
}

func TestFollow_DMUnfollowFollow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello @alice@localhost.localdomain:8443")

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	dm := server.Handle("/users/dm?Hello%20%40alice%40localhost.localdomain%3a8443", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello @alice@localhost.localdomain:8443")

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello @alice@localhost.localdomain:8443")

	unfollow := server.Handle("/users/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello @alice@localhost.localdomain:8443")
}

func TestFollow_PublicPost(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	whisper := server.Handle("/users/say?Hello%20world", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello world")
}

func TestFollow_Mutual(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	whisper := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	reply := server.Handle(fmt.Sprintf("/users/reply/%s?Hello%%20Alice", id), server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, reply)

	users = server.Handle("/users", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Alice)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello Alice")

	users = server.Handle("/users", server.Bob)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello world")

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/users", server.Bob)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello world")
}

func TestFollow_AlreadyFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal("40 Already following https://localhost.localdomain:8443/user/bob\r\n", follow)
}

func TestFollow_NoSuchUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/localhost.localdomain:8443/user/erin", server.Alice)
	assert.Equal("40 No such user\r\n", follow)
}

func TestFollow_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/localhost.localdomain:8443/user/erin", nil)
	assert.Equal("30 /users\r\n", follow)
}
