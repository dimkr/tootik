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

package cluster

import (
	"crypto/ed25519"
	"testing"

	"github.com/btcsuite/btcutil/base58"
)

func TestCluster_ReplyForwardingWithIntegrityProofs(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

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

func TestCluster_ReplyForwardingWithoutIntegrityProofs(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	// a.localdomain don't attach proofs to outgoing activities and c.localdomain should fetch forwarded activities
	cluster["a.localdomain"].Config.DisableIntegrityProofs = true

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

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

func TestCluster_ReplyForwardingPortableActors(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	_, alicePriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key for alice: %v", err)
	}
	alicePrivBase58 := "z" + base58.Encode(append([]byte{0x80, 0x26}, alicePriv.Seed()...))

	_, bobPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key for bob: %v", err)
	}
	bobPrivBase58 := "z" + base58.Encode(append([]byte{0x80, 0x26}, bobPriv.Seed()...))

	_, carolPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key for bob: %v", err)
	}
	carolPrivBase58 := "z" + base58.Encode(append([]byte{0x80, 0x26}, carolPriv.Seed()...))

	alice := cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+alicePrivBase58).OK()
	bob := cluster["b.localdomain"].Handle(bobKeypair, "/users/register?"+bobPrivBase58).OK()
	carol := cluster["c.localdomain"].Handle(carolKeypair, "/users/register?"+carolPrivBase58).OK()

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
