/*
Copyright 2025, 2026 Dima Krasner

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

package fed

import (
	"crypto/ed25519"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/migrations"
	"github.com/dimkr/tootik/proof"
	"github.com/dimkr/tootik/sqlite"
	"github.com/stretchr/testify/assert"
)

type portableTestActor struct {
	DID  string
	ID   string
	Priv ed25519.PrivateKey
}

func newPortableTestActor(t *testing.T) portableTestActor {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)
	return portableTestActor{DID: did, ID: "ap://" + did + "/actor", Priv: priv}
}

// document returns a signed actor document with the given ID, owned by a, served by the given gateways.
func (a portableTestActor) document(t *testing.T, id string, gateways []string, signer ed25519.PrivateKey) string {
	actor := ap.Actor{
		Context:           []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v1"},
		ID:                id,
		Type:              ap.Person,
		PreferredUsername: "alice",
		Inbox:             id + "/inbox",
		Outbox:            id + "/outbox",
		Followers:         id + "/followers",
		Gateways:          gateways,
		AssertionMethod: []ap.AssertionMethod{
			{
				ID:                 id + "#ed25519-key",
				Type:               "Multikey",
				Controller:         id,
				PublicKeyMultibase: a.DID[len("did:key:"):],
			},
		},
	}

	if signer != nil {
		var err error
		if actor.Proof, err = proof.Create(httpsig.Key{ID: id + "#ed25519-key", PrivateKey: signer}, &actor); err != nil {
			t.Fatalf("Failed to sign actor: %v", err)
		}
	}

	j, err := json.Marshal(actor)
	if err != nil {
		t.Fatalf("Failed to marshal actor: %v", err)
	}

	return string(j)
}

func newPortableTestResolver(t *testing.T, client *testClient) (*Resolver, [3]httpsig.Key, *sql.DB) {
	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })

	db, err := sql.Open(sqlite.DriverName, sqlite.Scheme+f.Name()+"?"+sqlite.JournalModeWAL)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	if err := migrations.Run(t.Context(), "localhost.localdomain", db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	var cfg cfg.Config
	cfg.FillDefaults()
	cfg.MinActorAge = 0

	_, key, err := user.CreateApplicationActor(t.Context(), "localhost.localdomain", db, &cfg)
	if err != nil {
		t.Fatalf("Failed to create application actor: %v", err)
	}

	return NewResolver(&BlockList{}, "localhost.localdomain", &cfg, client, db), key, db
}

func TestResolve_PortableActorUsingHint(t *testing.T) {
	assert := assert.New(t)

	alice := newPortableTestActor(t)

	client := newTestClient(map[string]testResponse{
		"https://gw1.example/.well-known/apgateway/" + alice.DID + "/actor": {
			Response: newTestResponse(http.StatusOK, alice.document(t, alice.ID, []string{"https://gw2.example", "https://gw1.example"}, alice.Priv)),
		},
	})

	resolver, key, db := newPortableTestResolver(t, &client)

	actor, err := resolver.ResolveID(t.Context(), key, alice.ID+"?@gateway=https%3A%2F%2Fgw1.example", 0)
	assert.NoError(err)
	assert.Empty(client.Data)
	assert.Equal(alice.ID, actor.ID)

	var id, cid string
	assert.NoError(db.QueryRowContext(t.Context(), `select id, cid from persons where cid = ?`, alice.ID).Scan(&id, &cid))
	assert.Equal("https://gw2.example/.well-known/apgateway/"+alice.DID+"/actor", id)
	assert.Equal(alice.ID, cid)

	for _, cached := range []string{
		alice.ID,
		"ap+ef61://did%3Akey%3A" + alice.DID[len("did:key:"):] + "/actor",
		"https://gw1.example/.well-known/apgateway/" + alice.DID + "/actor",
		"https://gw3.example/.well-known/apgateway/" + alice.DID + "/actor",
		alice.ID + "#ed25519-key",
	} {
		actor, err := resolver.ResolveID(t.Context(), key, cached, ap.Offline)
		assert.NoError(err, cached)
		assert.Equal(alice.ID, actor.ID, cached)
	}
}

func TestResolve_PortableActorGatewayFailover(t *testing.T) {
	assert := assert.New(t)

	alice := newPortableTestActor(t)

	client := newTestClient(map[string]testResponse{
		"https://gw1.example/.well-known/apgateway/" + alice.DID + "/actor": {
			Error: errors.New("gw1 is down"),
		},
		"https://gw2.example/.well-known/apgateway/" + alice.DID + "/actor": {
			Response: newTestResponse(http.StatusOK, alice.document(t, alice.ID, []string{"https://gw1.example", "https://gw2.example"}, alice.Priv)),
		},
	})

	resolver, key, _ := newPortableTestResolver(t, &client)

	actor, err := resolver.ResolveID(t.Context(), key, alice.ID+"?@gateway=https%3A%2F%2Fgw1.example&@gateway=https%3A%2F%2Fgw2.example", 0)
	assert.NoError(err)
	assert.Empty(client.Data)
	assert.Equal(alice.ID, actor.ID)
}

func TestResolve_PortableActorNoGateways(t *testing.T) {
	assert := assert.New(t)

	alice := newPortableTestActor(t)

	client := newTestClient(map[string]testResponse{})

	resolver, key, _ := newPortableTestResolver(t, &client)

	_, err := resolver.ResolveID(t.Context(), key, alice.ID, 0)
	assert.Error(err)
}

func TestResolve_PortableActorInvalid(t *testing.T) {
	alice := newPortableTestActor(t)
	bob := newPortableTestActor(t)

	for _, tc := range []struct {
		name     string
		document string
	}{
		{"no proof", alice.document(t, alice.ID, []string{"https://gw1.example"}, nil)},
		{"proof by another key", alice.document(t, alice.ID, []string{"https://gw1.example"}, bob.Priv)},
		{"another DID", bob.document(t, bob.ID, []string{"https://gw1.example"}, bob.Priv)},
		{"no gateways", alice.document(t, alice.ID, nil, alice.Priv)},
		{"local gateway", alice.document(t, alice.ID, []string{"https://localhost.localdomain"}, alice.Priv)},
		{"gateway with path", alice.document(t, alice.ID, []string{"https://gw1.example/x"}, alice.Priv)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)

			client := newTestClient(map[string]testResponse{
				"https://gw1.example/.well-known/apgateway/" + alice.DID + "/actor": {
					Response: newTestResponse(http.StatusOK, tc.document),
				},
			})

			resolver, key, db := newPortableTestResolver(t, &client)

			_, err := resolver.ResolveID(t.Context(), key, alice.ID+"?@gateway=https%3A%2F%2Fgw1.example", 0)
			assert.Error(err)
			assert.Empty(client.Data)

			var count int
			assert.NoError(db.QueryRowContext(t.Context(), `select count(*) from persons where cid like 'ap://did:key:%' and ed25519seed is null`).Scan(&count))
			assert.Zero(count)
		})
	}
}
