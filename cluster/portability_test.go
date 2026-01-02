/*
Copyright 2025 Dima Krasner

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

package cluster

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func TestCluster_ReplyForwardingPortableActors(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].RegisterPortable(aliceKeypair).OK()
	bob := cluster["b.localdomain"].RegisterPortable(bobKeypair).OK()
	carol := cluster["c.localdomain"].RegisterPortable(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	reply := alice.GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	cluster.Settle(t)

	bob = bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})
	alice.
		Follow("😈 My profile").
		Contains(Line{Type: Quote, Text: "hi"})
	carol = carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})

	reply.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	bob.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	alice.
		Follow("😈 My profile").
		Contains(Line{Type: Quote, Text: "hola"})
	carol.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})

	reply.Follow("💣 Delete").OK()
	cluster.Settle(t)

	bob.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	alice.
		Follow("😈 My profile").
		NotContains(Line{Type: Quote, Text: "hola"})
	carol.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_Gateways(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].RegisterPortable(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	bob.
		Follow("⚡️ Follows").
		Contains(Line{Type: Link, Text: "🚴 alice (alice@a.localdomain)", URL: "/users/outbox/a.localdomain/.well-known/apgateway/" + did + "/actor"})

	post := alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi").
		OK()
	carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})

	bob.
		FollowInput("🔭 View profile", "carol@c.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	bob.GotoInput(post.Links["💬 Reply"], "hola").
		Contains(Line{Type: Quote, Text: "hola"})
	cluster.Settle(t)

	alice.
		Goto(post.Path).
		Contains(Line{Type: Quote, Text: "hola"})

	carol.
		Goto(post.Path).
		Contains(Line{Type: Quote, Text: "hola"})

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(Line{Type: Quote, Text: "hola"})

	carol.GotoInput(post.Links["🩹 Edit"], "yo").
		Contains(Line{Type: Quote, Text: "yo"})
	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "yo"})

	carol.Goto(post.Links["💣 Delete"])
	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "yo"})
}

func TestCluster_ForwardedLegacyReply(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	cluster["b.localdomain"].Config.RFC9421Threshold = 1
	cluster["b.localdomain"].Config.Ed25519Threshold = 1
	cluster["b.localdomain"].Config.DisableIntegrityProofs = true

	alice := cluster["a.localdomain"].RegisterPortable(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].RegisterPortable(carolKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	post := alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	bob.GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	cluster.Settle(t)

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningInboxHappyFlow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Create,
		ID:        actorID + "/create/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object: &ap.Object{
			Type:         ap.Note,
			ID:           actorID + "/note/1",
			Content:      "hi",
			AttributedTo: actorID,
			To:           to,
			CC:           to,
		},
	}

	create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: priv}, create)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/inbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusAccepted {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxHappyFlow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Create,
		ID:        actorID + "/create/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object: &ap.Object{
			Type:         ap.Note,
			ID:           actorID + "/note/1",
			Content:      "hi",
			AttributedTo: actorID,
			To:           to,
			CC:           to,
		},
	}

	create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: priv}, create)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/"+did+"/actor/outbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusAccepted {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxWrongOutbox(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Create,
		ID:        actorID + "/create/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object: &ap.Object{
			Type:         ap.Note,
			ID:           actorID + "/note/1",
			Content:      "hi",
			AttributedTo: actorID,
			To:           to,
			CC:           to,
		},
	}

	create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: priv}, create)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/did:key:z6MktUDwghnPi5JDt1j274ukswcfD9oeJmmCAhcyDgsEy79y/actor/outbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusBadRequest {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxUnsupportedActivity(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Add,
		ID:        actorID + "/add/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object:    1234,
	}

	create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: priv}, create)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/"+did+"/actor/outbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusBadRequest {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxNotActivity(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	to := ap.Audience{}
	to.Add(ap.Public)

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/"+did+"/actor/outbox", strings.NewReader("123"))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusInternalServerError {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxInvalidActivity(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Reject,
		ID:        actorID + "/create/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object: &ap.Object{
			Type:         ap.Note,
			ID:           actorID + "/note/1",
			Content:      "hi",
			AttributedTo: actorID,
			To:           to,
			CC:           to,
		},
	}

	create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: priv}, create)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/"+did+"/actor/outbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusBadRequest {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxNoProof(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Create,
		ID:        actorID + "/create/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object: &ap.Object{
			Type:         ap.Note,
			ID:           actorID + "/note/1",
			Content:      "hi",
			AttributedTo: actorID,
			To:           to,
			CC:           to,
		},
	}

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/"+did+"/actor/outbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusForbidden {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxInvalidVerificationMethod(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Create,
		ID:        actorID + "/create/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object: &ap.Object{
			Type:         ap.Note,
			ID:           actorID + "/note/1",
			Content:      "hi",
			AttributedTo: actorID,
			To:           to,
			CC:           to,
		},
	}

	create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: priv}, create)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	create.Proof.VerificationMethod = "abcd"

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/"+did+"/actor/outbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusForbidden {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_ClientSideSigningOutboxInvalidProof(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, registerPortable).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "c.localdomain").
		OK()

	carol.
		Follow("⚙️ Settings").
		Follow("🚲 Data portability").
		FollowInput("➕ Add", "a.localdomain").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	actorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	to := ap.Audience{}
	to.Add(ap.Public)

	create := &ap.Activity{
		Type:      ap.Create,
		ID:        actorID + "/create/1",
		Actor:     actorID,
		To:        to,
		CC:        to,
		Published: ap.Time{Time: time.Now()},
		Object: &ap.Object{
			Type:         ap.Note,
			ID:           actorID + "/note/1",
			Content:      "hi",
			AttributedTo: actorID,
			To:           to,
			CC:           to,
		},
	}

	create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: priv}, create)
	if err != nil {
		t.Fatalf("Failed to generate proof: %v", err)
	}

	create.Proof.Created = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)

	j, err := json.Marshal(create)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://c.localdomain/.well-known/apgateway/"+did+"/actor/outbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var w responseWriter
	cluster["c.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusForbidden {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hi"})
}

func TestCluster_InboxFetchHappyFlow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+did+"/actor/inbox", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + did + "/actor#ed25519-key",
			PrivateKey: priv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to fetch inbox: %d", w.StatusCode)
	}

	var inbox ap.Collection
	if err := json.NewDecoder(&w.Body).Decode(&inbox); err != nil {
		t.Fatalf("Failed to decode inbox: %v", err)
	}

	r, err = http.NewRequestWithContext(t.Context(), http.MethodGet, inbox.First, nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + did + "/actor#ed25519-key",
			PrivateKey: priv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w = responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to fetch inbox page: %d", w.StatusCode)
	}

	var page struct {
		OrderedItems []ap.Activity `json:"orderedItems"`
	}
	if err := json.NewDecoder(&w.Body).Decode(&page); err != nil {
		t.Fatalf("Failed to decode inbox page: %v", err)
	}
}

func TestCluster_InboxFetchInvalidSignature(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+did+"/actor/inbox", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	var wrongPriv ed25519.PrivateKey
	_, wrongPriv, err = ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + did + "/actor#ed25519-key",
			PrivateKey: wrongPriv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusNotFound {
		t.Fatalf("Failed to fetch inbox: %d", w.StatusCode)
	}
}

func TestCluster_InboxFetchNotOwner(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain")
	defer cluster.Stop()

	alicePub, alicePriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	aliceDID := "did:key:" + data.EncodeEd25519PublicKey(alicePub)
	alice := cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(alicePriv)).OK()

	bobPub, bobPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	bobDID := "did:key:" + data.EncodeEd25519PublicKey(bobPub)
	bob := cluster["a.localdomain"].Handle(bobKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(bobPriv)).OK()

	alice.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+aliceDID+"/actor/inbox", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + bobDID + "/actor#ed25519-key",
			PrivateKey: bobPriv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusNotFound {
		t.Fatalf("Failed to fetch inbox: %d", w.StatusCode)
	}
}

func TestCluster_InboxFetchNotRegistered(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	_, alicePriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	alice := cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(alicePriv)).OK()

	bobPub, bobPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	bobDID := "did:key:" + data.EncodeEd25519PublicKey(bobPub)
	bob := cluster["b.localdomain"].Handle(bobKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(bobPriv)).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+bobDID+"/actor/inbox", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + bobDID + "/actor#ed25519-key",
			PrivateKey: bobPriv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusNotFound {
		t.Fatalf("Failed to fetch inbox: %d", w.StatusCode)
	}
}

func TestCluster_OutboxImport(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerPortable := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Handle(aliceKeypair, registerPortable).OK()
	bob := cluster["b.localdomain"].Handle(bobKeypair, registerPortable).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "carol@a.localdomain").
		Follow("⚡ Follow carol").
		OK()
	cluster.Settle(t)

	alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi").
		OK()
	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"})

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+did+"/actor/outbox", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + did + "/actor#ed25519-key",
			PrivateKey: priv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to fetch inbox: %d", w.StatusCode)
	}

	var inbox ap.Collection
	if err := json.NewDecoder(&w.Body).Decode(&inbox); err != nil {
		t.Fatalf("Failed to decode inbox: %v", err)
	}

	r, err = http.NewRequestWithContext(t.Context(), http.MethodGet, inbox.First, nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + did + "/actor#ed25519-key",
			PrivateKey: priv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w = responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to fetch inbox page: %d", w.StatusCode)
	}

	var page struct {
		OrderedItems []json.RawMessage `json:"orderedItems"`
	}
	if err := json.NewDecoder(&w.Body).Decode(&page); err != nil {
		t.Fatalf("Failed to decode inbox page: %v", err)
	}

	for _, item := range page.OrderedItems {
		r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://b.localdomain/.well-known/apgateway/"+did+"/actor/outbox", bytes.NewReader([]byte(item)))
		if err != nil {
			t.Fatalf("Failed to create HTTP request: %v", err)
		}

		var w responseWriter
		cluster["b.localdomain"].Backend.ServeHTTP(&w, r)
		if w.StatusCode != http.StatusAccepted {
			t.Fatalf("Failed to import activity: %d", w.StatusCode)
		}
	}

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})
}

func TestCluster_ClientSideSigningFollowersHappyFlow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alicePub, alicePriv, err := ed25519.GenerateKey(nil)
	aliceDID := "did:key:" + data.EncodeEd25519PublicKey(alicePub)
	alice := cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(alicePriv)).OK()

	bobPub, bobPriv, err := ed25519.GenerateKey(nil)
	bobDID := "did:key:" + data.EncodeEd25519PublicKey(bobPub)
	bob := cluster["b.localdomain"].Handle(bobKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(bobPriv)).OK()

	carolPub, carolPriv, err := ed25519.GenerateKey(nil)
	carolDID := "did:key:" + data.EncodeEd25519PublicKey(carolPub)
	carol := cluster["c.localdomain"].Handle(carolKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(carolPriv)).OK()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+aliceDID+"/actor/followers", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://a.localdomain/.well-known/apgateway/" + aliceDID + "/actor#ed25519-key",
			PrivateKey: alicePriv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	var followers struct {
		OrderedItems []string `json:"orderedItems"`
	}
	if err := json.NewDecoder(&w.Body).Decode(&followers); err != nil {
		t.Fatalf("Failed to decode followers: %v", err)
	}

	if followers.OrderedItems == nil || len(followers.OrderedItems) > 0 {
		t.Fatalf("Unexpected list of followers: %v", followers.OrderedItems)
	}

	alice.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice (requires approval)").
		OK()
	cluster.Settle(t)

	alice.
		Follow("🐕 Followers").
		Follow("🟢 Accept")
	cluster.Settle(t)

	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice (requires approval)").
		OK()
	cluster.Settle(t)

	w = responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	if err := json.NewDecoder(&w.Body).Decode(&followers); err != nil {
		t.Fatalf("Failed to decode followers: %v", err)
	}

	if !slices.Equal(
		followers.OrderedItems,
		[]string{
			"https://b.localdomain/.well-known/apgateway/" + bobDID + "/actor",
		},
	) {
		t.Fatalf("Unexpected list of followers: %v", followers.OrderedItems)
	}

	alice.
		Follow("🐕 Followers").
		Follow("🟢 Accept")
	cluster.Settle(t)

	w = responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	if err := json.NewDecoder(&w.Body).Decode(&followers); err != nil {
		t.Fatalf("Failed to decode followers: %v", err)
	}

	if !slices.Equal(
		followers.OrderedItems,
		[]string{
			"https://b.localdomain/.well-known/apgateway/" + bobDID + "/actor",
			"https://c.localdomain/.well-known/apgateway/" + carolDID + "/actor",
		},
	) {
		t.Fatalf("Unexpected list of followers: %v", followers.OrderedItems)
	}

	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("🔌 Unfollow alice").
		OK()
	cluster.Settle(t)

	w = responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusOK {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}

	if err := json.NewDecoder(&w.Body).Decode(&followers); err != nil {
		t.Fatalf("Failed to decode followers: %v", err)
	}

	if !slices.Equal(
		followers.OrderedItems,
		[]string{
			"https://b.localdomain/.well-known/apgateway/" + bobDID + "/actor",
		},
	) {
		t.Fatalf("Unexpected list of followers: %v", followers.OrderedItems)
	}
}

func TestCluster_ClientSideSigningFollowersMissingCollection(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	_, alicePriv, err := ed25519.GenerateKey(nil)
	cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(alicePriv)).OK()

	bobPub, bobPriv, err := ed25519.GenerateKey(nil)
	bobDID := "did:key:" + data.EncodeEd25519PublicKey(bobPub)
	cluster["b.localdomain"].Handle(bobKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(bobPriv)).OK()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+bobDID+"/actor/followers", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://b.localdomain/.well-known/apgateway/" + bobDID + "/actor#ed25519-key",
			PrivateKey: bobPriv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusNotFound {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}
}

func TestCluster_ClientSideSigningFollowersWrongCollection(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alicePub, alicePriv, err := ed25519.GenerateKey(nil)
	aliceDID := "did:key:" + data.EncodeEd25519PublicKey(alicePub)
	cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(alicePriv)).OK()

	bobPub, bobPriv, err := ed25519.GenerateKey(nil)
	bobDID := "did:key:" + data.EncodeEd25519PublicKey(bobPub)
	cluster["b.localdomain"].Handle(bobKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(bobPriv)).OK()

	r, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "https://a.localdomain/.well-known/apgateway/"+aliceDID+"/actor/followers", nil)
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	if err := httpsig.SignRFC9421(
		r,
		nil,
		httpsig.Key{
			ID:         "https://b.localdomain/.well-known/apgateway/" + bobDID + "/actor#ed25519-key",
			PrivateKey: bobPriv,
		},
		time.Now(),
		time.Now().Add(time.Minute*5),
		httpsig.RFC9421DigestSHA256,
		"ed25519",
		nil,
	); err != nil {
		t.Fatalf("Failed to sign HTTP request: %v", err)
	}

	w := responseWriter{
		Headers: http.Header{},
	}
	cluster["a.localdomain"].Backend.ServeHTTP(&w, r)
	if w.StatusCode != http.StatusNotFound {
		t.Fatalf("Failed to process activity: %d", w.StatusCode)
	}
}
