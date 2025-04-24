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

import "testing"

func TestCluster_PostToFollowers_Approved(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK().
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle()

	alice.
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain)", URL: "/users/outbox/b.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle()

	carol.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔓 Approve follow requests manually").
		Contains(Line{Type: Text, Text: "No follow requests."}).
		Contains(Line{Type: Link, Text: "🔒 Approve follow requests automatically", URL: "/users/follows/pending?disable"})

	carol.
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"}).
		Follow("⚡ Follow carol (requires approval)").
		OK().
		NotContains(Line{Type: Quote, Text: "hello"}).
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle()

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"}).
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})

	carol.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🟢 Accept").
		Contains(Line{Type: Text, Text: "No follow requests."})
	cluster.Settle()

	bob.
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain)", URL: "/users/outbox/b.localdomain/user/carol"}).
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Link, Text: "🔌 Unfollow carol", URL: "/users/unfollow/b.localdomain/user/carol"}).
		Contains(Line{Type: Quote, Text: "hello"}).
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_Rejected(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK().
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle()

	alice.
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain)", URL: "/users/outbox/b.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle()

	carol.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔓 Approve follow requests manually").
		Contains(Line{Type: Text, Text: "No follow requests."}).
		Contains(Line{Type: Link, Text: "🔒 Approve follow requests automatically", URL: "/users/follows/pending?disable"})

	carol.
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"}).
		Follow("⚡ Follow carol (requires approval)").
		OK().
		NotContains(Line{Type: Quote, Text: "hello"}).
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle()

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"}).
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})

	carol.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔴 Reject").
		Contains(Line{Type: Text, Text: "No follow requests."})
	cluster.Settle()

	bob.
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - rejected", URL: "/users/outbox/b.localdomain/user/carol"}).
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(Line{Type: Link, Text: "🔌 Unfollow carol (rejected)", URL: "/users/unfollow/b.localdomain/user/carol"}).
		NotContains(Line{Type: Quote, Text: "hello"}).
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_DisabledThenAccepted(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	carol.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔓 Approve follow requests manually").
		Contains(Line{Type: Text, Text: "No follow requests."}).
		Contains(Line{Type: Link, Text: "🔒 Approve follow requests automatically", URL: "/users/follows/pending?disable"})

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol (requires approval)").
		OK().
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle()

	carol.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔒 Approve follow requests automatically").
		Contains(Line{Type: Link, Text: "🔓 Approve follow requests manually", URL: "/users/follows/pending?enable"})
	cluster.Settle()

	alice.
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK().
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle()

	bob.
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@b.localdomain)", URL: "/users/outbox/b.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle()

	alice.
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})
	bob.
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})

	carol.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🟢 Accept").
		Contains(Line{Type: Text, Text: "No follow requests."})
	cluster.Settle()

	alice.
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_ApprovedLocally(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔓 Approve follow requests manually").
		Contains(Line{Type: Text, Text: "No follow requests."}).
		Contains(Line{Type: Link, Text: "🔒 Approve follow requests automatically", URL: "/users/follows/pending?disable"})

	alice.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob (requires approval)").
		OK().
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "😈 bob (bob@a.localdomain) - pending approval", URL: "/users/outbox/a.localdomain/user/bob"})

	bob.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})

	alice.
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🟢 Accept").
		Contains(Line{Type: Text, Text: "No follow requests."})

	cluster.Settle()

	alice.
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})

	carol.
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔒 Approve follow requests automatically").
		Contains(Line{Type: Text, Text: "No follow requests."}).
		Contains(Line{Type: Link, Text: "🔓 Approve follow requests manually", URL: "/users/follows/pending?enable"})

	carol.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle()

	carol.
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_RejectedLocally(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔓 Approve follow requests manually").
		Contains(Line{Type: Text, Text: "No follow requests."}).
		Contains(Line{Type: Link, Text: "🔒 Approve follow requests automatically", URL: "/users/follows/pending?disable"})

	alice.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob (requires approval)").
		OK().
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "😈 bob (bob@a.localdomain) - pending approval", URL: "/users/outbox/a.localdomain/user/bob"})

	bob.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(Line{Type: Quote, Text: "hello"})

	alice.
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🔴 Reject").
		Contains(Line{Type: Text, Text: "No follow requests."})

	cluster.Settle()

	alice.
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})

	carol.
		Follow("📻 My feed").
		NotContains(Line{Type: Quote, Text: "hello"})

	carol.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob (requires approval)").
		OK()

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Follow requests").
		Follow("🟢 Accept").
		Contains(Line{Type: Text, Text: "No follow requests."})
	cluster.Settle()

	carol.
		Follow("📻 My feed").
		Contains(Line{Type: Quote, Text: "hello"})
}
