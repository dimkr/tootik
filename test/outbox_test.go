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
	"net/http"
	"strings"
	"testing"

	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/inbox"
	"github.com/stretchr/testify/assert"
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

	dm := server.Handle("/users/dm?Hello%20%40bob", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Contains(outbox, "Hello @bob")
}

func TestOutbox_DMSelf(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	dm := server.Handle("/users/dm?Hello%20%40bob", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Alice)
	assert.Contains(outbox, "Hello @bob")
}

func TestOutbox_DMNotRecipient(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	dm := server.Handle("/users/dm?Hello%20%40bob", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/users/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Carol)
	assert.NotContains(outbox, "Hello @bob")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	dm := server.Handle("/users/dm?Hello%20%40bob", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, dm)

	outbox := server.Handle("/outbox/"+strings.TrimPrefix(server.Alice.ID, "https://"), nil)
	assert.NotContains(outbox, "Hello @bob")
	assert.Contains(strings.Split(outbox, "\n"), "No posts.")
}

func TestOutbox_PublicPostInGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}
`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/outbox/other.localdomain/group/people", nil)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupAudienceSetByUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://127.0.0.1/user/dan",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello world")

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox = server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupAudienceSetByGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://127.0.0.1/user/dan",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/2","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox = server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupDeletedByUser(t *testing.T) {
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
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/2","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://127.0.0.1/user/dan",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/delete/1","type":"Delete","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox = server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupDeletedByAnotherUser(t *testing.T) {
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

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/2","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://127.0.0.1/user/erin",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/delete/1","type":"Delete","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox = server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupDeletedByGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/2","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/announce/1","type":"Announce","object":{"id":"https://127.0.0.1/delete/1","type":"Delete","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"audience":"https://other.localdomain/group/people"}`,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox = server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupForwardedDelete(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/erin",
		`{"type":"Person","preferredUsername":"erin","followers":"https://127.0.0.1/followers/erin"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://127.0.0.1/user/erin",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://https://127.0.0.1/announce/2","type":"Announce","object":{"id":"https://127.0.0.1/delete/1","type":"Delete","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"audience":"https://other.localdomain/group/people"}`,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox = server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PublicPostInGroupEditedByUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","published":"2018-08-18T00:00:00Z","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello world")

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://127.0.0.1/user/dan",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://127.0.0.1/update/1","type":"Update","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello again","published":"2018-08-18T00:00:00Z","updated":"2088-08-18T00:00:00Z","to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	n, err = queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox = server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.Contains(outbox, "Hello again")
}

func TestOutbox_PostToFollowersInGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersInGroupNotFollowingGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersInGroupNotAccepted(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersInGroupFollowingAuthor(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/127.0.0.1/user/dan", server.Alice)
	assert.Equal("30 /users/outbox/127.0.0.1/user/dan\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_PostToFollowersInGroupUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":[],"cc":["https://127.0.0.1/followers/dan","https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.Contains(outbox, "Hello world")
}

func TestOutbox_DMInGroupNotFollowingGroup(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Alice)
	assert.NotContains(outbox, "Hello world")
}

func TestOutbox_DMInGroupAnotherUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	_, err := server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://127.0.0.1/user/dan",
		`{"type":"Person","preferredUsername":"dan","followers":"https://127.0.0.1/followers/dan"}`,
	)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into persons (id, actor) values(?,?)`,
		"https://other.localdomain/group/people",
		`{"id":"https://other.localdomain/group/people","type":"Group","preferredUsername":"people"}`,
	)
	assert.NoError(err)

	follow := server.Handle("/users/follow/other.localdomain/group/people", server.Alice)
	assert.Equal("30 /users/outbox/other.localdomain/group/people\r\n", follow)

	follow = server.Handle("/users/follow/"+strings.TrimPrefix(server.Alice.ID, "https://"), server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), follow)

	_, err = server.db.Exec(`update follows set accepted = 1`)
	assert.NoError(err)

	_, err = server.db.Exec(
		`insert into inbox (sender, activity, raw) values($1, $2, $2)`,
		"https://other.localdomain/group/people",
		`{"@context":["https://www.w3.org/ns/activitystreams"],"id":"https://other.localdomain/announce/1","type":"Announce","actor":"https://other.localdomain/group/people","object":{"id":"https://127.0.0.1/create/1","type":"Create","actor":"https://127.0.0.1/user/dan","object":{"id":"https://127.0.0.1/note/1","type":"Note","attributedTo":"https://127.0.0.1/user/dan","content":"Hello world","to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://other.localdomain/group/people"],"audience":"https://other.localdomain/group/people"},"to":["https://localhost.localdomain:8443/user/alice"],"cc":["https://other.localdomain/group/people"]},"to":["https://www.w3.org/ns/activitystreams#Public"],"cc":["https://other.localdomain/group/people"]}`,
	)
	assert.NoError(err)

	queue := inbox.Queue{
		Domain:    domain,
		Config:    server.cfg,
		BlockList: &fed.BlockList{},
		DB:        server.db,
		Resolver:  fed.NewResolver(nil, domain, server.cfg, &http.Client{}, server.db),
		Key:       server.NobodyKey,
	}
	n, err := queue.ProcessBatch(context.Background())
	assert.NoError(err)
	assert.Equal(1, n)

	outbox := server.Handle("/users/outbox/other.localdomain/group/people", server.Bob)
	assert.NotContains(outbox, "Hello world")
}
