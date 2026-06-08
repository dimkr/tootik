/*
Copyright 2024 - 2026 Dima Krasner

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
	"testing"

	"github.com/dimkr/tootik/gemtext"
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
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hello"})
	cluster.Settle(t)

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
	cluster.Settle(t)

	alice.
		Refresh().
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
	bob.
		Refresh().
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.
		Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
	bob.
		Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
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
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hello"})
	cluster.Settle(t)

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hello"})

	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	alice.Refresh().
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
	bob.Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
	bob.Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
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
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "@alice@a.localdomain hello"})
	cluster.Settle(t)

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "@alice@a.localdomain hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "@alice@a.localdomain hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	alice.Refresh().
		Contains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
	bob.Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
	bob.Refresh().
		NotContains(gemtext.Line{Type: gemtext.Quote, Text: "hola"})
}
