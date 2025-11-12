/*
Copyright 2024, 2025 Dima Krasner

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

package httpsig

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"math/big"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSign_HappyFlow(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
}

func TestSign_Get(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodGet, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	sig, err := Extract(req, nil, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
}

func TestSign_NoKeyID(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.Error(t, Sign(req, Key{PrivateKey: priv}, now))
}

func TestSign_WrongKeyType(t *testing.T) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.Error(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))
}

func TestSign_MissingHeader(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.Error(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))
}

func TestSign_ReadFailure(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", &closedPipe{})
	assert.NoError(t, err)

	req.ContentLength = 1
	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.Error(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))
}

func TestSign_SignFailure(t *testing.T) {
	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.Error(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: &rsa.PrivateKey{PublicKey: rsa.PublicKey{N: big.NewInt(1)}}}, now))
}
