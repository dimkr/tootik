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
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFederated_AuthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	federated := server.Handle("/users/federated", server.Alice)
	assert.Regexp(t, "^20 text/gemini\r\n", federated)
}

func TestFederated_UnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	federated := server.Handle("/federated", nil)
	assert.Regexp(t, "^20 text/gemini\r\n", federated)
}

func TestFederated_InvalidOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	federated := server.Handle("/users/federated?a", server.Alice)
	assert.Equal(t, "40 Invalid query\r\n", federated)
}

func TestFederated_BigOffset(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	federated := server.Handle("/users/federated?901", server.Alice)
	assert.Equal(t, "40 Offset must be <= 900\r\n", federated)
}

func TestFederated_SecondPage(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	federated := server.Handle("/users/federated?30", server.Alice)
	assert.Regexp(t, "^20 text/gemini\r\n", federated)
}

func TestFederated_SecondPageUnauthenticatedUser(t *testing.T) {
	server := newTestServer()
	defer server.Shutdown()

	federated := server.Handle("/federated?30", nil)
	assert.Regexp(t, "^20 text/gemini\r\n", federated)
}
