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

func TestCluster_Hashtag(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	alice.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi #world").
		Contains(Line{Type: Quote, Text: "hi #world"}).
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi #peoplE").
		Contains(Line{Type: Quote, Text: "hi #peoplE"})

	post := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello #world").
		Contains(Line{Type: Quote, Text: "hello #world"})
	cluster.Settle(t)

	alice = alice.
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "world").
		Contains(Line{Type: Quote, Text: "hi #world"}).
		Contains(Line{Type: Quote, Text: "hello #world"})

	post.FollowInput("🩹 Edit", "hello #world #people").
		Contains(Line{Type: Quote, Text: "hello #world #people"})
	cluster.Settle(t)

	alice.
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "world").
		Contains(Line{Type: Quote, Text: "hi #world"}).
		Contains(Line{Type: Quote, Text: "hello #world #people"}).
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "people").
		Contains(Line{Type: Quote, Text: "hi #peoplE"}).
		Contains(Line{Type: Quote, Text: "hello #world #people"})

	post.FollowInput("🩹 Edit", "hello #world #people #PeOpLe").
		Contains(Line{Type: Quote, Text: "hello #world #people #PeOpLe"})
	cluster.Settle(t)

	alice.
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "world").
		Contains(Line{Type: Quote, Text: "hi #world"}).
		Contains(Line{Type: Quote, Text: "hello #world #people #PeOpLe"}).
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "people").
		Contains(Line{Type: Quote, Text: "hi #peoplE"}).
		Contains(Line{Type: Quote, Text: "hello #world #people #PeOpLe"}).
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "PEOPLE").
		Contains(Line{Type: Quote, Text: "hi #peoplE"}).
		Contains(Line{Type: Quote, Text: "hello #world #people #PeOpLe"})

	post.FollowInput("🩹 Edit", "hello #people").
		Contains(Line{Type: Quote, Text: "hello #people"})
	cluster.Settle(t)

	alice.
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "people").
		Contains(Line{Type: Quote, Text: "hi #peoplE"}).
		Contains(Line{Type: Quote, Text: "hello #people"})

	post.Follow("💣 Delete").OK()
	cluster.Settle(t)

	alice.
		Follow("🔥 Hashtags").
		FollowInput("🔎 Posts by hashtag", "people").
		Contains(Line{Type: Quote, Text: "hi #peoplE"}).
		NotContains(Line{Type: Quote, Text: "hello #people"})
}
