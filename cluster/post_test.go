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
	"context"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/user"
	"github.com/dimkr/tootik/outbox"
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

func TestCluster_PostInCommunity(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "g.localdomain")
	defer cluster.Stop()

	if _, _, err := user.Create(context.Background(), "g.localdomain", cluster["g.localdomain"].DB, "stuff", ap.Group, nil); err != nil {
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
	cluster.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@stuff@g.localdomain hello").
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	cluster.Settle()

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
	cluster.Settle()

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
	cluster.Settle()

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

	if _, _, err := user.Create(context.Background(), "g.localdomain", cluster["g.localdomain"].DB, "stuff", ap.Group, nil); err != nil {
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
	cluster.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@stuff@g.localdomain hello").
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	cluster.Settle()

	reply := alice.
		GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	cluster.Settle()

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
	cluster.Settle()

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
	cluster.Settle()

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

func TestCluster_ReplyForwarding(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	alice := cluster["b.localdomain"].Register(aliceKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle()

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle()

	reply := alice.GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	cluster.Settle()

	bob = bob.
		FollowInput("🔭 View profile", "alice@b.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})
	alice.
		Follow("😈 My profile").
		Contains(Line{Type: Quote, Text: "hi"})
	carol = carol.
		FollowInput("🔭 View profile", "alice@b.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})

	reply.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle()

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
	cluster.Settle()

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

func TestCluster_ShareUnshare(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	alice = alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol = carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle()

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle()

	share := alice.Goto(post.Path).
		Follow("🔁 Share").
		OK()
	cluster.Settle()

	bob = bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})
	alice.
		Refresh().
		Contains(Line{Type: Quote, Text: "hello"})
	carol.
		Refresh().
		Contains(Line{Type: Quote, Text: "hello"})

	share.Follow("🔄️ Unshare").OK()
	cluster.Settle()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"})
	alice.
		Follow("😈 My profile").
		NotContains(Line{Type: Quote, Text: "hello"})
	carol.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hello"})
}

func TestCluster_Poll(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	alice = alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol = carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle()

	poll := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "[POLL Favorite color] Gray | Orange").
		OK()
	cluster.Settle()

	bob = poll.Follow("📮 Vote Orange").OK()
	alice = alice.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	carol = carol.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	cluster.Settle()

	poller := outbox.Poller{
		Domain: "b.localdomain",
		DB:     cluster["b.localdomain"].DB,
		Config: cluster["b.localdomain"].Config,
	}
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle()

	bob.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "2 ████████ Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████     Orange"})
	alice.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "2 ████████ Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████     Orange"})
	carol.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "2 ████████ Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████     Orange"})

	alice.Follow("💣 Delete").OK()
	cluster.Settle()
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle()

	bob.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Orange"})
	alice.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Orange"})
	carol.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Orange"})

	carol.Follow("💣 Delete").OK()
	cluster.Settle()
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle()

	bob.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "0          Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Orange"})
	alice.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "0          Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Orange"})
	carol.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "0          Gray"}).
		Contains(Line{Type: Preformatted, Text: "1 ████████ Orange"})

	bob.
		Follow("💣 Delete").
		OK()
	cluster.Settle()
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle()

	bob.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "0          Gray"}).
		Contains(Line{Type: Preformatted, Text: "0          Orange"})
	alice.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "0          Gray"}).
		Contains(Line{Type: Preformatted, Text: "0          Orange"})
	carol.
		Goto(poll.Path).
		Contains(Line{Type: Preformatted, Text: "0          Gray"}).
		Contains(Line{Type: Preformatted, Text: "0          Orange"})
}
