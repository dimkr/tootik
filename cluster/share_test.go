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

import "testing"

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
