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
	"strings"
	"testing"

	"github.com/dimkr/tootik/inbox"
	"github.com/stretchr/testify/assert"
)

func TestUnfollow_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	say := server.Handle("/login/whisper?Hello%20followers", server.Bob)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "Hello followers")

	unfollow := server.Handle("/login/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/login", server.Alice)
	assert.Contains(users, "Hello followers")
}

func TestUnfollow_HappyFlowBeforeFeedUpdate(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	say := server.Handle("/login/whisper?Hello%20followers", server.Bob)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	users := server.Handle("/login", server.Alice)
	assert.NotContains(users, "Hello followers")

	unfollow := server.Handle("/login/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/login", server.Alice)
	assert.NotContains(users, "Hello followers")
}

func TestUnfollow_FollowAgain(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	say := server.Handle("/login/whisper?Hello%20followers", server.Bob)
	assert.Regexp(`^30 /login/view/\S+\r\n$`, say)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "Hello followers")

	unfollow := server.Handle("/login/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), unfollow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/login", server.Alice)
	assert.Contains(users, "Hello followers")

	follow = server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/login", server.Alice)
	assert.Contains(users, "Hello followers")
}

func TestUnfollow_NotFollowing(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	unfollow := server.Handle("/login/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal("40 No such follow\r\n", unfollow)
}

func TestUnfollow_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	unfollow := server.Handle("/login/unfollow/"+strings.TrimPrefix(server.Bob.ID, "https://"), nil)
	assert.Equal("30 /login\r\n", unfollow)
}
