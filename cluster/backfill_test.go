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

func TestCluster_BackfillMissingParent(t *testing.T) {
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
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"})

	reply := alice.GotoInput(post.Links["💬 Reply"], "hi").
		Contains(Line{Type: Quote, Text: "hi"})
	cluster.Settle(t)

	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hi"})

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	cluster["c.localdomain"].Config.BackfillInterval = 0

	reply.FollowInput("🩹 Edit", "hola").OK()
	cluster.Settle(t)

	carol.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "hola"})

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		NotContains(Line{Type: Quote, Text: "hello"})
}

func TestCluster_BackfillDeletedParentSameServer(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].RegisterPortable(aliceKeypair).OK()
	bob := cluster["a.localdomain"].RegisterPortable(bobKeypair).OK()
	carol := cluster["b.localdomain"].RegisterPortable(carolKeypair).OK()

	carol.
		FollowInput("🔭 View profile", "bob@a.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	head := alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "a").
		OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "a"})

	deleted := alice.
		GotoInput(head.Links["💬 Reply"], "b").
		Contains(Line{Type: Quote, Text: "b"})

	reply := bob.
		GotoInput(deleted.Links["💬 Reply"], "c").
		Contains(Line{Type: Quote, Text: "c"})

	deleted.
		Follow("💣 Delete").
		OK()

	cluster.Settle(t)

	carol.
		Goto(reply.Path).
		Contains(Line{Type: Quote, Text: "a"}).
		NotContains(Line{Type: Quote, Text: "b"}).
		Contains(Line{Type: Quote, Text: "c"})
}

func TestCluster_BackfillDeletedParentDifferentServer(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].RegisterPortable(aliceKeypair).OK()
	bob := cluster["b.localdomain"].RegisterPortable(bobKeypair).OK()
	carol := cluster["c.localdomain"].RegisterPortable(carolKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	head := alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "a").
		OK()
	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(Line{Type: Quote, Text: "a"})

	deleted := alice.
		GotoInput(head.Links["💬 Reply"], "b").
		Contains(Line{Type: Quote, Text: "b"})
	cluster.Settle(t)

	deleted.
		Follow("💣 Delete").
		OK()
	reply := bob.
		GotoInput(deleted.Links["💬 Reply"], "c").
		Contains(Line{Type: Quote, Text: "c"})
	cluster.Settle(t)

	carol.
		Goto(reply.Path).
		Contains(Line{Type: Quote, Text: "a"}).
		NotContains(Line{Type: Quote, Text: "b"}).
		Contains(Line{Type: Quote, Text: "c"})
}
