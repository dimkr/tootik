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
	"crypto/ed25519"
	"testing"

	"github.com/dimkr/tootik/data"
)

func TestCluster_ReplyForwardingPortableActors(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	cluster["a.localdomain"].Config.EnablePortableActorRegistration = true
	cluster["b.localdomain"].Config.EnablePortableActorRegistration = true
	cluster["c.localdomain"].Config.EnablePortableActorRegistration = true

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

	cluster["a.localdomain"].Config.EnablePortableActorRegistration = true
	cluster["b.localdomain"].Config.EnablePortableActorRegistration = true
	cluster["c.localdomain"].Config.EnablePortableActorRegistration = true

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

	cluster["a.localdomain"].Config.EnablePortableActorRegistration = true
	cluster["b.localdomain"].Config.RFC9421Threshold = 1
	cluster["b.localdomain"].Config.Ed25519Threshold = 1
	cluster["b.localdomain"].Config.DisableIntegrityProofs = true
	cluster["c.localdomain"].Config.EnablePortableActorRegistration = true

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
