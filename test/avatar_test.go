/*
Copyright 2024 Dima Krasner

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
	"github.com/dimkr/tootik/ap"
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
	"time"
)

var avatar = []byte("\x47\x49\x46\x38\x37\x61\x10\x00\x10\x00\xf0\x00\x00\x00\x00\x00\xff\x00\x00\x2c\x00\x00\x00\x00\x10\x00\x10\x00\x00\x02\x1e\x8c\x8f\xa9\xab\xe0\x0f\x1d\x8a\x14\xcc\x0a\x2f\x96\x67\x3f\xbd\x81\x98\x58\x91\x94\x19\xa1\x59\xe7\x59\xcc\x0b\x2b\x05\x00\x3b")

func TestAvatar_HappyFlow(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal(fmt.Sprintf("30 gemini://localhost.localdomain:8443/users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), server.Upload("/users/upload/avatar;mime=image/gif;size=63", server.Alice, avatar))
}

func TestAvatar_NewUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published = &ap.Time{Time: time.Now().Add(-time.Second * 5)}
	assert.Regexp(`^40 Please wait for \S+\r\n$`, server.Upload("/users/upload/avatar;mime=image/gif;size=63", server.Alice, avatar))
}

func TestAvatar_ChangedRecently(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	server.Alice.Updated = &ap.Time{Time: time.Now().Add(-time.Second * 5)}
	assert.Regexp(`^40 Please wait for \S+\r\n$`, server.Upload("/users/upload/avatar;mime=image/gif;size=63", server.Alice, avatar))
}

func TestAvatar_HappyFlowSizeFirst(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal(fmt.Sprintf("30 gemini://localhost.localdomain:8443/users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), server.Upload("/users/upload/avatar;size=63;mime=image/gif", server.Alice, avatar))
}

func TestAvatar_InvalidSize(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal("40 Invalid size\r\n", server.Upload("/users/upload/avatar;mime=image/gif;size=abc", server.Alice, avatar))
}

func TestAvatar_InvalidType(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal("40 Unsupported image type\r\n", server.Upload("/users/upload/avatar;mime=text/plain;size=63", server.Alice, avatar))
}

func TestAvatar_NoSize(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal("40 Error\r\n", server.Upload("/users/upload/avatar;mime=image/gif;ize=63", server.Alice, avatar))
}

func TestAvatar_NoType(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal("40 Error\r\n", server.Upload("/users/upload/avatar;mim=image/gif;size=63", server.Alice, avatar))
}

func TestAvatar_InvalidImage(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal("40 Error\r\n", server.Upload("/users/upload/avatar;mime=image/gif;size=3", server.Alice, []byte("abc")))
}

func TestAvatar_TooSmallSize(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal("40 Error\r\n", server.Upload("/users/upload/avatar;mime=image/gif;size=10", server.Alice, avatar))
}

func TestAvatar_TooBigSize(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	assert.Equal("40 Error\r\n", server.Upload("/users/upload/avatar;mime=image/gif;size=64", server.Alice, avatar))
}

func TestAvatar_SizeLimit(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	server.cfg.MaxAvatarSize = 62
	assert.Equal("40 Image is too big\r\n", server.Upload("/users/upload/avatar;mime=image/gif;size=63", server.Alice, avatar))
}

func TestAvatar_ExactlySizeLimit(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	server.Alice.Published.Time = server.Alice.Published.Time.Add(-time.Hour)
	server.cfg.MaxAvatarSize = 63
	assert.Equal(fmt.Sprintf("30 gemini://localhost.localdomain:8443/users/outbox/%s\r\n", strings.TrimPrefix(server.Alice.ID, "https://")), server.Upload("/users/upload/avatar;mime=image/gif;size=63", server.Alice, avatar))
}
