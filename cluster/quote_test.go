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

import (
	"strings"
	"testing"
)

func TestCluster_PublicPostQuote(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	cluster.Settle(t)

	profile := alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})

	quoted := false
	for desc, url := range profile.Links {
		if strings.HasPrefix(url, "/users/view/") {
			profile.
				Follow(desc).
				FollowInput("♻️ Quote", "hola").
				Contains(Line{Type: Quote, Text: "hola"}).
				Contains(Line{Type: Quote, Text: "hello"})

			quoted = true
			break
		}
	}

	if !quoted {
		t.Fatal("Post not found")
	}

	cluster.Settle(t)

	post.
		Refresh().
		Follow("♻️ alice").
		Contains(Line{Type: Quote, Text: "hola"}).
		Contains(Line{Type: Quote, Text: "hello"})
}
