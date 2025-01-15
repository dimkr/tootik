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
	f := NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["a.localdomain"].Register(bobKeypair).OK()
	carol := f["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK()
	f.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	f.Settle()

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").
		Contains(Line{Type: Quote, Text: "hola"})
	f.Settle()

	alice.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.
		Refresh().
		Contains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	f.Settle()

	alice.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.
		Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_PostToFollowers(t *testing.T) {
	f := NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["a.localdomain"].Register(bobKeypair).OK()
	carol := f["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK()
	f.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	f.Settle()

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	f.Settle()

	alice.Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	f.Settle()

	alice.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_DM(t *testing.T) {
	f := NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["a.localdomain"].Register(bobKeypair).OK()
	carol := f["b.localdomain"].Register(carolKeypair).OK()

	post := carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("📣 New post").
		FollowInput("💌 Mentioned users only", "@alice@a.localdomain hello").
		Contains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})
	f.Settle()

	alice = alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})
	bob = bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "@alice@a.localdomain hello"})

	post.FollowInput("🩹 Edit", "hola").OK()
	f.Settle()

	alice.Refresh().
		Contains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})

	post.Follow("💣 Delete").OK()
	f.Settle()

	alice.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
	bob.Refresh().
		NotContains(Line{Type: Quote, Text: "hola"})
}

func TestCluster_PostInCommunity(t *testing.T) {
	f := NewFediverse(t, "a.localdomain", "b.localdomain", "g.localdomain")
	defer f.Stop()

	if _, _, err := user.Create(context.Background(), "g.localdomain", f["g.localdomain"].DB, "stuff", ap.Group, nil); err != nil {
		t.Fatal("Failed to create community")
	}

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["a.localdomain"].Register(bobKeypair).OK()
	carol := f["b.localdomain"].Register(carolKeypair).OK()

	alice = alice.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()

	carol = carol.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()
	f.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@stuff@g.localdomain hello").
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	f.Settle()

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
	f.Settle()

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
	f.Settle()

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
	f := NewFediverse(t, "a.localdomain", "b.localdomain", "g.localdomain")
	defer f.Stop()

	if _, _, err := user.Create(context.Background(), "g.localdomain", f["g.localdomain"].DB, "stuff", ap.Group, nil); err != nil {
		t.Fatal("Failed to create community")
	}

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["a.localdomain"].Register(bobKeypair).OK()
	carol := f["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()

	carol.
		FollowInput("🔭 View profile", "stuff@g.localdomain").
		Follow("⚡ Follow stuff").
		OK()
	f.Settle()

	post := carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@stuff@g.localdomain hello").
		Contains(Line{Type: Quote, Text: "@stuff@g.localdomain hello"})
	f.Settle()

	reply := alice.
		GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	f.Settle()

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
	f.Settle()

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
	f.Settle()

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
	f := NewFediverse(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer f.Stop()

	bob := f["a.localdomain"].Register(bobKeypair).OK()
	alice := f["b.localdomain"].Register(aliceKeypair).OK()
	carol := f["c.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	f.Settle()

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	f.Settle()

	reply := alice.GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	f.Settle()

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
	f.Settle()

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
	f.Settle()

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
	f := NewFediverse(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["b.localdomain"].Register(bobKeypair).OK()
	carol := f["c.localdomain"].Register(carolKeypair).OK()

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
	f.Settle()

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	f.Settle()

	share := alice.Goto(post.Path).
		Follow("🔁 Share").
		OK()
	f.Settle()

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
	f.Settle()

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
	f := NewFediverse(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["b.localdomain"].Register(bobKeypair).OK()
	carol := f["c.localdomain"].Register(carolKeypair).OK()

	alice = alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol = carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	f.Settle()

	poll := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "[POLL Favorite color] Gray | Orange").
		OK()
	f.Settle()

	bob = poll.Follow("📮 Vote Orange").OK()
	alice = alice.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	carol = carol.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	f.Settle()

	poller := outbox.Poller{
		Domain: "b.localdomain",
		DB:     f["b.localdomain"].DB,
		Config: f["b.localdomain"].Config,
	}
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	f.Settle()

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
	f.Settle()
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	f.Settle()

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
	f.Settle()
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	f.Settle()

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
	f.Settle()
	if err := poller.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	f.Settle()

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
