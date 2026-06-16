/*
Copyright 2025, 2026 Dima Krasner

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

	"github.com/dimkr/tootik/front/text/gmi"
)

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
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle(t)

	alice.
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain)", URL: "/users/outbox/b.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
	cluster.Settle(t)

	carol.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		Contains(gmi.Line{Type: gmi.Link, Text: "🔓 Approve new follow requests automatically", URL: "/users/followers?unlock"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"})

	carol.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("⚡ Follow carol (requires approval)").
		OK().
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	carol.
		Follow("🐕 Followers").
		Follow("🟢 Accept").
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/bob"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/bob"})
	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(gmi.Line{Type: gmi.Link, Text: "🔌 Unfollow carol", URL: "/users/unfollow/b.localdomain/user/carol"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_Rejected(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK().
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle(t)

	bob.
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain)", URL: "/users/outbox/b.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
	cluster.Settle(t)

	carol.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		Contains(gmi.Line{Type: gmi.Link, Text: "🔓 Approve new follow requests automatically", URL: "/users/followers?unlock"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/bob"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/bob"})

	carol.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("⚡ Follow carol (requires approval)").
		OK().
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	carol.
		Goto("/users/followers/reject/a.localdomain/user/alice").
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/bob"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/bob"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"})
	cluster.Settle(t)

	alice.
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - rejected", URL: "/users/outbox/b.localdomain/user/carol"}).
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Contains(gmi.Line{Type: gmi.Link, Text: "🔌 Unfollow carol (rejected)", URL: "/users/unfollow/b.localdomain/user/carol"}).
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"}).
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_DisabledThenAccepted(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	carol.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		Contains(gmi.Line{Type: gmi.Text, Text: "No follow requests."}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔓 Approve new follow requests automatically", URL: "/users/followers?unlock"})

	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol (requires approval)").
		OK().
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle(t)

	carol.
		Follow("🐕 Followers").
		Follow("🔓 Approve new follow requests automatically").
		Contains(gmi.Line{Type: gmi.Link, Text: "🔒 Approve new follow requests manually", URL: "/users/followers?lock"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"})
	cluster.Settle(t)

	alice.
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})

	bob.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK().
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain) - pending approval", URL: "/users/outbox/b.localdomain/user/carol"})
	cluster.Settle(t)

	bob.
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@b.localdomain)", URL: "/users/outbox/b.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
	cluster.Settle(t)

	alice.
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})
	bob.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	carol.
		Follow("🐕 Followers").
		Follow("🟢 Accept").
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"})
	cluster.Settle(t)

	alice.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_ApprovedLocally(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	bob.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		Contains(gmi.Line{Type: gmi.Text, Text: "No follow requests."}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔓 Approve new follow requests automatically", URL: "/users/followers?unlock"})

	alice.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob (requires approval)").
		OK().
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "😈 bob (bob@a.localdomain) - pending approval", URL: "/users/outbox/a.localdomain/user/bob"})

	bob.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	alice.
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	bob.
		Follow("🐕 Followers").
		Follow("🟢 Accept").
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"})

	cluster.Settle(t)

	alice.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	carol.
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	bob.
		Follow("🐕 Followers").
		Follow("🔓 Approve new follow requests automatically").
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/carol"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/carol"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔒 Approve new follow requests manually", URL: "/users/followers?lock"})

	carol.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	carol.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_RejectedLocally(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["a.localdomain"].Register(bobKeypair).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	bob.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		Contains(gmi.Line{Type: gmi.Text, Text: "No follow requests."}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔓 Approve new follow requests automatically", URL: "/users/followers?unlock"})

	alice.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob (requires approval)").
		OK().
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "😈 bob (bob@a.localdomain) - pending approval", URL: "/users/outbox/a.localdomain/user/bob"})

	bob.
		Follow("📣 New post").
		FollowInput("🔔 Your followers and mentioned users", "hello").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	alice.
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	bob.
		Follow("🐕 Followers").
		Follow("🔴 Reject").
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"})

	cluster.Settle(t)

	alice.
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	carol.
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	carol.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob (requires approval)").
		OK()

	bob.
		Follow("🐕 Followers").
		Follow("🟢 Accept").
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/alice"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/alice"}).
		NotContains(gmi.Line{Type: gmi.Link, Text: "🟢 Accept", URL: "/users/followers/accept/a.localdomain/user/carol"}).
		Contains(gmi.Line{Type: gmi.Link, Text: "🔴 Reject", URL: "/users/followers/reject/a.localdomain/user/carol"})
	cluster.Settle(t)

	carol.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
}

func TestCluster_PostToFollowers_AcceptTwice(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice (requires approval)").
		OK()
	cluster.Settle(t)

	pending := alice.
		Follow("🐕 Followers").
		OK()

	pending.
		Follow("🟢 Accept").
		OK()

	pending.
		Follow("🟢 Accept").
		Error("40 No such follow request")
}

func TestCluster_PostToFollowers_RejectTwice(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice (requires approval)").
		OK()
	cluster.Settle(t)

	pending := alice.
		Follow("🐕 Followers").
		OK()

	pending.
		Follow("🔴 Reject").
		OK()

	pending.
		Follow("🔴 Reject").
		Error("40 Error")
}

func TestCluster_PostToFollowers_AcceptThenReject(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice (requires approval)").
		OK()
	cluster.Settle(t)

	pending := alice.
		Follow("🐕 Followers").
		OK()

	pending.
		Follow("🟢 Accept").
		OK()

	pending.
		Follow("🔴 Reject").
		OK()
}

func TestCluster_PostToFollowers_RejectThenAccept(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		Follow("🐕 Followers").
		Follow("🔒 Approve new follow requests manually").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice (requires approval)").
		OK()
	cluster.Settle(t)

	pending := alice.
		Follow("🐕 Followers").
		OK()

	pending.
		Follow("🔴 Reject").
		OK()

	pending.
		Follow("🟢 Accept").
		Error("40 No such follow request")
}
