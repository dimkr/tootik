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
	"context"
	"crypto/tls"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/fedtest"
	"github.com/dimkr/tootik/front/user"
	_ "github.com/mattn/go-sqlite3"
)

const (
	/*
	   openssl ecparam -name prime256v1 -genkey -out /tmp/ec.pem
	   openssl req -new -x509 -key /tmp/ec.pem -sha256 -nodes -subj "/CN=carol" -out cert.pem -keyout key.pem -days 3650
	*/
	carolCert = `-----BEGIN CERTIFICATE-----
MIIBdjCCARugAwIBAgIUeZy9BQQp+bnEzoD5TDRv2xLSZuYwCgYIKoZIzj0EAwIw
EDEOMAwGA1UEAwwFY2Fyb2wwHhcNMjQxMjE0MTMxODIzWhcNMzQxMjEyMTMxODIz
WjAQMQ4wDAYDVQQDDAVjYXJvbDBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABKPN
YqQIl/sjMezo3ZttCv8qH2ntwyStzSvaJezcrBUCtElXxaKa0ZOZr+6U0DEKNa4X
iIuZMGVAkrLu3suQZwqjUzBRMB0GA1UdDgQWBBRi3uJanIOqk6RcBoTGY9FZYeZZ
wzAfBgNVHSMEGDAWgBRi3uJanIOqk6RcBoTGY9FZYeZZwzAPBgNVHRMBAf8EBTAD
AQH/MAoGCCqGSM49BAMCA0kAMEYCIQDNsWvggf21RLAm76emHlUY6jcwtkeKS8LR
ffR/EWG5tQIhAIJuoPkZl7/1UNrrnPfg8y2viY3FqOzOf4ReaCyWUfcS
-----END CERTIFICATE-----`

	carolKey = `-----BEGIN PRIVATE KEY-----
MIGHAgEAMBMGByqGSM49AgEGCCqGSM49AwEHBG0wawIBAQQgrbcQ9pVVzG20eUou
XUQVNWPSQLogHZo2Zk4IVeVjgiihRANCAASjzWKkCJf7IzHs6N2bbQr/Kh9p7cMk
rc0r2iXs3KwVArRJV8WimtGTma/ulNAxCjWuF4iLmTBlQJKy7t7LkGcK
-----END PRIVATE KEY-----`
)

var carol, david, erin tls.Certificate

func init() {
	var err error
	carol, err = tls.X509KeyPair([]byte(carolCert), []byte(carolKey))
	if err != nil {
		panic(err)
	}

	david, err = tls.X509KeyPair([]byte(davidCert), []byte(davidKey))
	if err != nil {
		panic(err)
	}

	erin, err = tls.X509KeyPair([]byte(erinCert), []byte(erinKey))
	if err != nil {
		panic(err)
	}
}

func TestFederation_PublicPost(t *testing.T) {
	f := fedtest.NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	a := f["a.localdomain"]
	b := f["b.localdomain"]

	a.Handle(carol, "/users/register").OK()
	a.Handle(david, "/users/register").OK()
	b.Handle(erin, "/users/register").OK()

	a.Handle(carol, "/users/resolve?erin%40b.localdomain").OK()
	a.Handle(carol, "/users/follow/b.localdomain/user/erin").OK()
	f.Settle()

	post := b.Handle(erin, "/users/say?hello").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})

	post.Follow("🩹 Edit", "hola").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})

	post.Follow("💣 Delete", "").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
}

func TestFederation_PostToFollowers(t *testing.T) {
	f := fedtest.NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	a := f["a.localdomain"]
	b := f["b.localdomain"]

	a.Handle(carol, "/users/register").OK()
	a.Handle(david, "/users/register").OK()
	b.Handle(erin, "/users/register").OK()

	a.Handle(carol, "/users/resolve?erin%40b.localdomain").OK()
	a.Handle(carol, "/users/follow/b.localdomain/user/erin").OK()
	f.Settle()

	post := b.Handle(erin, "/users/whisper?hello").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})

	post.Follow("🩹 Edit", "hola").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})

	post.Follow("💣 Delete", "").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
}

func TestFederation_DM(t *testing.T) {
	f := fedtest.NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	a := f["a.localdomain"]
	b := f["b.localdomain"]

	a.Handle(carol, "/users/register").OK()
	a.Handle(david, "/users/register").OK()
	b.Handle(erin, "/users/register").OK()

	b.Handle(erin, "/users/resolve?carol%40a.localdomain").OK()
	post := b.Handle(erin, "/users/dm?%40carol%40a.localdomain%20hello").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "@carol@a.localdomain hello"})
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "@carol@a.localdomain hello"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "@carol@a.localdomain hello"})

	post.Follow("🩹 Edit", "hola").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})

	post.Follow("💣 Delete", "").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/b.localdomain/user/erin").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
}

func TestFederation_PostInCommunity(t *testing.T) {
	f := fedtest.NewFediverse(t, "a.localdomain", "b.localdomain", "g.localdomain")
	defer f.Stop()

	a := f["a.localdomain"]
	b := f["b.localdomain"]
	g := f["g.localdomain"]

	if _, _, err := user.Create(context.Background(), "g.localdomain", g.DB, "stuff", ap.Group, nil); err != nil {
		t.Fatal("Failed to create community")
	}

	a.Handle(carol, "/users/register").OK()
	a.Handle(david, "/users/register").OK()
	b.Handle(erin, "/users/register").OK()

	a.Handle(carol, "/users/resolve?stuff%40g.localdomain").OK()
	a.Handle(carol, "/users/follow/g.localdomain/user/stuff").OK()
	f.Settle()

	b.Handle(erin, "/users/resolve?stuff%40g.localdomain").OK()
	b.Handle(erin, "/users/follow/g.localdomain/user/stuff").OK()
	f.Settle()

	post := b.Handle(erin, "/users/say?%40stuff%40g.localdomain%20hello").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "@stuff@g.localdomain hello"})
	f.Settle()

	a.Handle(carol, "/users/outbox/g.localdomain/user/stuff").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "@stuff@g.localdomain hello"})
	a.Handle(david, "/users/outbox/g.localdomain/user/stuff").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "@stuff@g.localdomain hello"})
	b.Handle(erin, "/users/outbox/g.localdomain/user/stuff").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "@stuff@g.localdomain hello"})

	post.Follow("🩹 Edit", "hola").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/g.localdomain/user/stuff").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/g.localdomain/user/stuff").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	b.Handle(erin, "/users/outbox/g.localdomain/user/stuff").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})

	post.Follow("💣 Delete", "").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/g.localdomain/user/stuff").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/g.localdomain/user/stuff").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	b.Handle(erin, "/users/outbox/g.localdomain/user/stuff").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
}

func TestFederation_ReplyInCommunity(t *testing.T) {
	f := fedtest.NewFediverse(t, "a.localdomain", "b.localdomain", "g.localdomain")
	defer f.Stop()

	a := f["a.localdomain"]
	b := f["b.localdomain"]
	g := f["g.localdomain"]

	if _, _, err := user.Create(context.Background(), "g.localdomain", g.DB, "stuff", ap.Group, nil); err != nil {
		t.Fatal("Failed to create community")
	}

	a.Handle(carol, "/users/register").OK()
	a.Handle(david, "/users/register").OK()
	b.Handle(erin, "/users/register").OK()

	a.Handle(carol, "/users/resolve?stuff%40g.localdomain").OK()
	a.Handle(carol, "/users/follow/g.localdomain/user/stuff").OK()
	f.Settle()

	b.Handle(erin, "/users/resolve?stuff%40g.localdomain").OK()
	b.Handle(erin, "/users/follow/g.localdomain/user/stuff").OK()
	f.Settle()

	post := b.Handle(erin, "/users/say?%40stuff%40g.localdomain%20hello").
		OK().
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "@stuff@g.localdomain hello"})
	f.Settle()

	reply := a.Handle(david, post.Links["💬 Reply"]+"?hi").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	f.Settle()

	a.Handle(carol, "/users/outbox/a.localdomain/user/david").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	a.Handle(david, "/users/outbox/a.localdomain/user/david").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	b.Handle(erin, "/users/outbox/a.localdomain/user/david").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})

	reply.Follow("🩹 Edit", "hola").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/a.localdomain/user/david").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	a.Handle(david, "/users/outbox/a.localdomain/user/david").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	b.Handle(erin, "/users/outbox/a.localdomain/user/david").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})

	reply.Follow("💣 Delete", "").OK()
	f.Settle()

	a.Handle(carol, "/users/outbox/a.localdomain/user/david").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	a.Handle(david, "/users/outbox/a.localdomain/user/david").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	b.Handle(erin, "/users/outbox/a.localdomain/user/david").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
}

func TestFederation_ReplyForwarding(t *testing.T) {
	f := fedtest.NewFediverse(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer f.Stop()

	a := f["a.localdomain"]
	b := f["b.localdomain"]
	c := f["c.localdomain"]

	a.Handle(david, "/users/register").OK()
	b.Handle(carol, "/users/register").OK()
	c.Handle(erin, "/users/register").OK()

	b.Handle(carol, "/users/resolve?david%40a.localdomain").OK()
	b.Handle(carol, "/users/follow/a.localdomain/user/david").OK()

	c.Handle(erin, "/users/resolve?david%40a.localdomain").OK()
	c.Handle(erin, "/users/follow/a.localdomain/user/david").OK()

	f.Settle()

	post := a.Handle(david, "/users/say?hello").
		OK().
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	f.Settle()

	reply := b.Handle(carol, post.Links["💬 Reply"]+"?hi").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	f.Settle()

	a.Handle(david, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	b.Handle(carol, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})
	c.Handle(erin, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hi"})

	reply.Follow("🩹 Edit", "hola").OK()
	f.Settle()

	a.Handle(david, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	b.Handle(carol, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	c.Handle(erin, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})

	reply.Follow("💣 Delete", "").OK()
	f.Settle()

	a.Handle(david, "/users/outbox/b.localdomain/user/carol").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	b.Handle(carol, "/users/outbox/b.localdomain/user/carol").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
	c.Handle(erin, "/users/outbox/b.localdomain/user/carol").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hola"})
}

func TestFederation_ShareUnshare(t *testing.T) {
	f := fedtest.NewFediverse(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer f.Stop()

	a := f["a.localdomain"]
	b := f["b.localdomain"]
	c := f["c.localdomain"]

	a.Handle(david, "/users/register").OK()
	b.Handle(carol, "/users/register").OK()
	c.Handle(erin, "/users/register").OK()

	b.Handle(carol, "/users/resolve?david%40a.localdomain").OK()
	b.Handle(carol, "/users/follow/a.localdomain/user/david").OK()

	c.Handle(erin, "/users/resolve?david%40a.localdomain").OK()
	c.Handle(erin, "/users/follow/a.localdomain/user/david").OK()

	c.Handle(erin, "/users/resolve?carol%40b.localdomain").OK()
	c.Handle(erin, "/users/follow/b.localdomain/user/carol").OK()

	f.Settle()

	post := a.Handle(david, "/users/say?hello").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	f.Settle()

	share := b.Handle(carol, post.Request).
		Follow("🔁 Share", "").
		OK()
	f.Settle()

	a.Handle(david, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	b.Handle(carol, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	c.Handle(erin, "/users/outbox/b.localdomain/user/carol").
		Contains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})

	share.Follow("🔄️ Unshare", "").OK()
	f.Settle()

	c.Handle(erin, "/users/outbox/b.localdomain/user/carol").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	c.Handle(erin, "/users/outbox/b.localdomain/user/carol").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
	c.Handle(erin, "/users/outbox/b.localdomain/user/carol").
		NotContains(fedtest.Line{Type: fedtest.Quote, Text: "hello"})
}
