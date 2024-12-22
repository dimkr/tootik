/*
Copyright 2024 Dima Krasner

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

package fedtest

import (
	"context"
	"testing"

	"github.com/dimkr/tootik/outbox"
)

func TestFederation_MovedAccount(t *testing.T) {
	f := NewFediverse(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["b.localdomain"].Register(bobKeypair).OK()
	carol := f["c.localdomain"].Register(carolKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	f.Settle()

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
	f.Settle()

	bob.FollowInput("🔭 View profile", "carol@c.localdomain").OK()

	mover := outbox.Mover{
		Domain:   "b.localdomain",
		DB:       f["b.localdomain"].DB,
		Resolver: f["b.localdomain"].Resolver,
		Key:      f["b.localdomain"].NobodyKey,
	}
	if err := mover.Run(context.Background()); err != nil {
		t.Fatalf("Failed to process moved accounts: %v", err)
	}
	f.Settle()

	bob.
		Follow("⚡️ Followed users").
		Contains(Line{Type: Link, Text: "👽 carol (carol@c.localdomain)", URL: "/users/outbox/c.localdomain/user/carol"})

	carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		Contains(Line{Type: Quote, Text: "hello"})
	f.Settle()

	bob.
		FollowInput("🔭 View profile", "carol@c.localdomain").
		Contains(Line{Type: Quote, Text: "hello"})
}
