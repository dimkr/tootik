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
