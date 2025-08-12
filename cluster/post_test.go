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

func TestCluster_PublicPost(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK()
	cluster.Settle(t)

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle(t)

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").
		Contains(Line{Type: Quote, Text: "hola"})
	cluster.Settle(t)

	alice.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_PostToFollowers(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK()
	cluster.Settle(t)

	post := carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle(t)

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	alice.Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_DM(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	post := carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("📣 New post").
		FollowInput("💌 Mentioned users only", "@alice@a.localdomain hello").
		Contains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})
	cluster.Settle(t)

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	alice.Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_Nomadic(t *testing.T) {
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

	nomadAlice := cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+alicePrivBase58).OK()
	nomadBob := cluster["b.localdomain"].Handle(bobKeypair, "/users/register?"+bobPrivBase58).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	nomadAlice.
		FollowInput("🔭 View profile", "carol@c.localdomain").
		Follow("⚡ Follow carol").
		OK()
	cluster.Settle(t)

	nomadBob.
		FollowInput("🔭 View profile", "carol@c.localdomain").
		Follow("⚡ Follow carol").
		OK()
	cluster.Settle(t)

	carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle(t)

	nomadAlice.
		FollowInput("🔭 View profile", "carol@c.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	carol.
		Follow("🐕 Followers").
		Follow("2025-08-12 👽 alice").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	nomadAlice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	cluster.Settle(t)

	carol.
		Follow("🐕 Followers").
		Follow("2025-08-12 👽 alice").
		Contains(Line{Type: Quote, Text: "hi"}).
		Follow("2025-08-12 alice").
		FollowInput("💬 Reply", "hola").
		Contains(Line{Type: Quote, Text: "hola"})
	cluster.Settle(t)

	nomadAlice.
		Follow("📻 My feed").
		Follow("2025-08-12 alice ┃ 1💬").
		Contains(Line{Type: Quote, Text: "hola"})

	nomadBob.
		Follow("📻 My feed").
		Follow("2025-08-12 alice ┃ 1💬").
		Contains(Line{Type: Quote, Text: "hola"})
}
