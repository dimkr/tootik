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
		Domain: "b.localdomain",
		DB:     cluster["b.localdomain"].DB,
		Inbox:  cluster["b.localdomain"].Inbox,
	}
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

	bob.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "2 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████     Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "2 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████     Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "2 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████     Orange"})

	alice.Follow("💣 Delete").OK()
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

	bob.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})

	carol.Follow("💣 Delete").OK()
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

	bob.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})

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
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
}

func TestCluster_PollVotersCount(t *testing.T) {
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

	aliceVote1 := alice.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	aliceVote2 := alice.
		Goto(poll.Links["📮 Vote Orange"]).
		OK()
	carolVote := carol.
		Goto(poll.Links["📮 Vote Gray"]).
		OK()
	cluster.Settle(t)

	poller := outbox.Poller{
		Domain: "b.localdomain",
		DB:     cluster["b.localdomain"].DB,
		Inbox:  cluster["b.localdomain"].Inbox,
	}
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

	bob.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (2 voters)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "2 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████     Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (2 voters)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "2 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████     Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (2 voters)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "2 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████     Orange"})

	carolVote.Follow("💣 Delete").OK()
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

	bob.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (one voter)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (one voter)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (one voter)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Orange"})

	aliceVote2.Follow("💣 Delete").OK()
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

	bob.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (one voter)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (one voter)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (one voter)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "1 ████████ Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})

	aliceVote1.Follow("💣 Delete").OK()
	cluster.Settle(t)
	if err := poller.Run(t.Context()); err != nil {
		t.Fatalf("Failed to process votes: %v", err)
	}
	cluster.Settle(t)

	bob.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (0 voters)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
	alice.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (0 voters)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
	carol.
		Goto(poll.Path).
		Contains(gmi.Line{Type: gmi.SubHeading, Text: "📊 Results (0 voters)"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Gray"}).
		Contains(gmi.Line{Type: gmi.Preformatted, Text: "0          Orange"})
}
