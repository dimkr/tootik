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
	"time"

	"github.com/dimkr/tootik/inbox"
	"github.com/stretchr/testify/assert"
)

func TestUploadEdit_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello followers")

	whisper := server.Handle("/login/whisper?Hello%20world", server.Bob)
	assert.Regexp(`30 /login/view/(\S+)\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Upload(fmt.Sprintf("/login/upload/edit/%s;mime=text/plain;size=15", id), server.Bob, []byte("Hello followers"))
	assert.Equal(fmt.Sprintf("30 gemini://%s/login/view/%s\r\n", domain, id), edit)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/login", server.Alice)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello followers")

	edit = server.Upload(fmt.Sprintf("/login/upload/edit/%s;mime=text/plain;size=16", id), server.Bob, []byte("Hello, followers"))
	assert.Regexp(`^40 Please wait for \S+\r\n$`, edit)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users = server.Handle("/login", server.Alice)
	assert.NotContains(users, "No posts.")
	assert.Contains(users, "Hello followers")
}

func TestUploadEdit_Empty(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello followers")

	whisper := server.Handle("/login/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /login/view/(\S+)\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Upload(fmt.Sprintf("/login/upload/edit/%s;mime=text/plain;size=0", id), server.Bob, []byte("Hello followers"))
	assert.Equal("40 Content is empty\r\n", edit)
}

func TestUploadEdit_SizeLimit(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello followers")

	whisper := server.Handle("/login/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /login/view/(\S+)\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	server.cfg.MaxPostsLength = 14

	edit := server.Upload(fmt.Sprintf("/login/upload/edit/%s;mime=text/plain;size=15", id), server.Bob, []byte("Hello followers"))
	assert.Equal("40 Post is too long\r\n", edit)
}

func TestUploadEdit_InvalidSize(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello followers")

	whisper := server.Handle("/login/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /login/view/(\S+)\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Upload(fmt.Sprintf("/login/upload/edit/%s;mime=text/plain;size=abc", id), server.Bob, []byte("Hello followers"))
	assert.Equal("40 Invalid size\r\n", edit)
}

func TestUploadEdit_InvalidType(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello followers")

	whisper := server.Handle("/login/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /login/view/(\S+)\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Upload(fmt.Sprintf("/login/upload/edit/%s;mime=text/gemini;size=15", id), server.Bob, []byte("Hello followers"))
	assert.Equal("40 Only text/plain is supported\r\n", edit)
}

func TestUploadEdit_NoSize(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello followers")

	whisper := server.Handle("/login/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /login/view/(\S+)\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Upload(fmt.Sprintf("/login/upload/edit/%s;mime=text/plain;siz=15", id), server.Bob, []byte("Hello followers"))
	assert.Equal("40 Invalid parameters\r\n", edit)
}

func TestUploadEdit_NoType(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	follow := server.Handle("/login/follow/"+strings.TrimPrefix(server.Bob.ID, "https://"), server.Alice)
	assert.Equal(fmt.Sprintf("30 /login/outbox/%s\r\n", strings.TrimPrefix(server.Bob.ID, "https://")), follow)

	assert.NoError((inbox.FeedUpdater{Domain: domain, Config: server.cfg, DB: server.db}).Run(context.Background()))

	users := server.Handle("/login", server.Alice)
	assert.Contains(users, "No posts.")
	assert.NotContains(users, "Hello followers")

	whisper := server.Handle("/login/whisper?Hello%20world", server.Bob)
	assert.Regexp(`^30 /login/view/(\S+)\r\n$`, whisper)

	id := whisper[15 : len(whisper)-2]

	_, err := server.db.Exec("update notes set inserted = inserted - 3600, object = json_set(object, '$.published', ?) where id = 'https://' || ?", time.Now().Add(-time.Hour).Format(time.RFC3339Nano), id)
	assert.NoError(err)

	edit := server.Upload(fmt.Sprintf("/login/upload/edit/%s;mim=text/plain;size=15", id), server.Bob, []byte("Hello followers"))
	assert.Equal("40 Invalid parameters\r\n", edit)
}
