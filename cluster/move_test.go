/*
Copyright 2024 - 2026 Dima Krasner

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
	"github.com/dimkr/tootik/outbox"
)

func TestCluster_MovedAccount(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	alice.
		Follow("⚙️ Settings").
		FollowInput("🔗 Set account alias", "carol@c.localdomain").
		OK()
	carol.
		Follow("⚙️ Settings").
		FollowInput("🔗 Set account alias", "alice@a.localdomain").
		OK()

	alice.
		Follow("⚙️ Settings").
		FollowInput("📦 Move account", "carol@c.localdomain").
		OK()
	cluster.Settle(t)

	bob.FollowInput("🔭 View profile", "carol@c.localdomain").OK()

	mover := outbox.Mover{
		Domain:   "b.localdomain",
		DB:       cluster["b.localdomain"].DB,
		Resolver: cluster["b.localdomain"].Resolver,
		Keys:     cluster["b.localdomain"].AppActorKeys,
		Inbox:    cluster["b.localdomain"].Inbox,
	}
	if err := mover.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process moved accounts: %v", err)
	}
	cluster.Settle(t)

	bob.
		Follow("⚡️ Follows").
		Contains(gmi.Line{Type: gmi.Link, Text: "👽 carol (carol@c.localdomain)", URL: "/users/outbox/c.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "carol@c.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
}

func TestCluster_DeletedInstance(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})
	cluster.Settle(t)

	alice.
		Follow("📻 My feed").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	cluster["b.localdomain"] = NewServer(t, "b.localdomain", Client{})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Error("40 Failed to resolve bob@b.localdomain")

	alice.
		Follow("📻 My feed").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello"})

	cluster.Settle(t)
}
