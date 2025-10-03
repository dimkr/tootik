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

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/user"
)

func TestCluster_PostInCommunity(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "g.localdomain")
	defer cluster.Stop()

	if _, _, err := user.Create(t.Context(), "g.localdomain", cluster["g.localdomain"].DB, cluster["g.localdomain"].Config, "stuff", ap.Group, nil); err != nil {
		t.Fatal("Failed to create community")
	}

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	alice = alice.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()

	carol = carol.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()
	cluster.Settle(t)

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@stuff@g.localdomain hello").
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	cluster.Settle(t)

	alice.
		Refresh().
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	bob = bob.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	carol.
		Refresh().
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	alice.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	carol.
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
	carol.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_ReplyInCommunity(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "g.localdomain")
	defer cluster.Stop()

	if _, _, err := user.Create(t.Context(), "g.localdomain", cluster["g.localdomain"].DB, cluster["g.localdomain"].Config, "stuff", ap.Group, nil); err != nil {
		t.Fatal("Failed to create community")
	}

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()

	carol.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()
	cluster.Settle(t)

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@stuff@g.localdomain hello").
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	cluster.Settle(t)

	reply := alice.
		GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})
	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})
	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})

	reply.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hola"})
	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hola"})
	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hola"})

	reply.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hola"})
	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hola"})
}
