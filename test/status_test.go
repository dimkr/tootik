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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatus_NewInstance(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	status := server.Handle("/status", server.Alice)
	assert.Regexp("^20 text/gemini\r\n", status)
}

func TestStatus_WithPosts(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	assert := assert.New(t)

	whisper := server.Handle("/users/whisper?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, whisper)

	status := server.Handle("/status", server.Alice)
	assert.Regexp("^20 text/gemini\r\n", status)
}
