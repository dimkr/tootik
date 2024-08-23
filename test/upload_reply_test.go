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

func TestUploadReply_PostToFollowers(t *testing.T) {
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

	reply := server.Upload(fmt.Sprintf("/users/upload/reply/%s;mime=text/plain;size=11", id), server.Alice, []byte("Welcome Bob"))
	assert.Regexp(fmt.Sprintf("^30 gemini://%s/users/view/\\S+\r\n$", domain), reply)

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
