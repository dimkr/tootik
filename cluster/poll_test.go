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
	"testing"

	"github.com/dimkr/tootik/outbox"
)

func TestCluster_Poll(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["c.localdomain"].Register(carolKeypair).OK()

	alice = alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	carol = carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	poll := bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "[POLL Favorite color] Gray | Orange").
		OK()
	cluster.Settle(t)

	bob = poll.Follow("📮 Vote Orange").OK()
	alice = alice.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	carol = carol.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	cluster.Settle(t)

	poller := outbox.Poller{
		DB:     cluster["b.localdomain"].DB,
		Domain: "b.localdomain",
		Inbox:  cluster["b.localdomain"].Incoming,
	}
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

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
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

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
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

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
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

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
