/*
Copyright 2024 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless ruired by applicable law or agreed to in writing, software
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
	"github.com/stretchr/testify/assert"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestVerify_TooOld(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now.Add(-time.Minute*2)))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_TooNew(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now.Add(time.Minute*2)))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_NoDate(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Del("Date")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_InvalidDate(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Date", "a")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_WrongHost(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	_, err = Extract(req, body, "wrong", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_EmptyHost(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Del("Host")
	req.Host = ""

	_, err = Extract(req, body, "wrong", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_NoHostFallback(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Del("Host")

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_NoHostWrongFallback(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Del("Host")

	_, err = Extract(req, body, "wrong", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_TwoSignatureHeaders(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Add("Signature", "")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_TwoKeyIDs(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", req.Header.Get("Signature")+`,keyId="a"`)

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_TwoSignatures(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", req.Header.Get("Signature")+`,signature="a"`)

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_TwoHeaders(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", req.Header.Get("Signature")+`,headers="a"`)

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_InvalidAttribute(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", req.Header.Get("Signature")+`,a="b"`)

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_NoKeyID(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), "keyId", "algorithm", 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_NoSignature(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), "signature", "algorithm", 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_NoHeaders(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), "headers", "algorithm", 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_InvalidSignatureBase64(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), `signature="`, `signature="a`, 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_DuplicateHeaders(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), `(request-target)`, `(request-target) (request-target)`, 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_HeadersOnlyWhitespace(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), `headers="`+strings.Join(postHeaders, " ")+`"`, `headers=" "`, 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_HeadersLeadingWhitespace(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", "\t\t"+req.Header.Get("Signature"))

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_HeadersTrailingWhitespace(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", req.Header.Get("Signature")+"\t\t")

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_HeadersContainsWhitespace(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), " content-type", " content-type\t\t", 1))

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_TargetNotSigned(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), "(request-target) ", "", 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_HostNotSigned(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), " host ", " ", 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_DateNotSigned(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), " date ", " ", 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_DigestNotSigned(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), ` digest"`, `"`, 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_MissingSignedHeader(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), `(request-target)`, `(request-target) aaa`, 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_MissingSpecialSignedHeader(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Signature", strings.Replace(req.Header.Get("Signature"), `(request-target)`, `(request-target) (request-aaa)`, 1))

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_DuplicateSignedHeader(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Add("Date", req.Header.Get("Date"))

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.Error(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_NoDigest(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Del("Digest")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_ShortDigest(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Digest", "a")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_InvalidDigestAlgorithm(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Digest", "SHA-512=a")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_InvalidDigestBase64(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Digest", req.Header.Get("Digest")+"a")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_InvalidDigestHashSize(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Digest", "SHA-256=DMF1ucDxtqgxw5niaXcmYQ==")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_WrongHash(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Digest", "SHA-256=ypeBEsobvcr6wjGzmiPcTaeG7/gUfE5yuYB3ha/uSLs=")

	_, err = Extract(req, body, "localhost", now, time.Minute)
	assert.Error(t, err)
}

func TestVerify_DifferentMethod(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Method = http.MethodGet

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.Error(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_DifferentHost(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://invalid/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://invalid/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Host", "localhost")

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.Error(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_DifferentDate(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Date", now.Add(-time.Second).UTC().Format(http.TimeFormat))

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.Error(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_DifferentContentType(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	body := []byte(`{"id":"a"}`)
	req, err := http.NewRequest(http.MethodPost, "http://localhost/inbox/nobody", bytes.NewReader(body))
	assert.NoError(t, err)

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	now := time.Now()
	assert.NoError(t, Sign(req, Key{ID: "http://localhost/key/nobody", PrivateKey: priv}, now))

	req.Header.Set("Content-Type", "text/plain")

	sig, err := Extract(req, body, "localhost", now, time.Minute)
	assert.NoError(t, err)

	assert.Error(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_WrongKey(t *testing.T) {
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

	priv2, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
	assert.Error(t, sig.Verify(&priv2.PublicKey))
}

func TestVerify_SmallKey(t *testing.T) {
	priv, err := rsa.GenerateKey(rand.Reader, 1024)
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

	assert.Error(t, sig.Verify(&priv.PublicKey))
}

func TestVerify_WrongKeyType(t *testing.T) {
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

	pub2, _, err := ed25519.GenerateKey(rand.Reader)
	assert.NoError(t, err)

	assert.NoError(t, sig.Verify(&priv.PublicKey))
	assert.Error(t, sig.Verify(pub2))
}
