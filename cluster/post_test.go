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
	"testing"
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
	cluster.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle()

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").
		Contains(Line{Type: Quote, Text: "hola"})
	cluster.Settle()

	alice.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle()

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
	cluster.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle()

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle()

	alice.Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle()

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
	cluster.Settle()

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle()

	alice.Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	cluster.Settle()

	alice.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}
