/*
Copyright 2026 Dima Krasner

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

import "testing"

func TestMention_ResolvedOP(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	post := alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "post").
		OK()
	cluster.Settle(t)

	reply1 := bob.
		GotoInput(post.Links["💬 Reply"], "@alice reply 1").
		OK()
	cluster.Settle(t)

	carol.
		GotoInput(reply1.Links["💬 Reply"], "@alice reply 2").
		Contains(Line{Type: Link, Text: "alice", URL: "/users/outbox/a.localdomain/user/alice"})
}

func TestMention_ResolvedFollow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@bob post").
		Contains(Line{Type: Link, Text: "bob", URL: "/users/outbox/b.localdomain/user/bob"})
}

func TestMention_ResolvedUserAndHost(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	cluster["b.localdomain"].Register(bobKeypair).OK()
	cluster["c.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	alice.
		FollowInput("🔭 View profile", "bob@c.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@bob@c.localdomain post").
		Contains(Line{Type: Link, Text: "bob", URL: "/users/outbox/c.localdomain/user/bob"})
}

func TestMention_Unresolved(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@bob post").
		Error("40 Unresolved mention: @bob")
}

func TestMention_AmbiguousTwoFollows(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	cluster["b.localdomain"].Register(bobKeypair).OK()
	cluster["c.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	alice.
		FollowInput("🔭 View profile", "bob@c.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@bob post").
		Error("40 Ambiguous mention: @bob")
}

func TestMention_AmbiguousFollowAndOP(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	cluster["b.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol.
		FollowInput("🔭 View profile", "alice@b.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	post := alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "post").
		OK()
	cluster.Settle(t)

	reply1 := bob.
		GotoInput(post.Links["💬 Reply"], "reply 1").
		OK()
	cluster.Settle(t)

	carol.
		GotoInput(reply1.Links["💬 Reply"], "@alice reply 2").
		Error("40 Ambiguous mention: @alice")
}

func TestMention_AmbiguousFollowAndLocal(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	cluster["a.localdomain"].Register(bobKeypair).OK()
	cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "@bob post").
		Error("40 Ambiguous mention: @bob")
}
