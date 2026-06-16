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
	"strings"
	"testing"

	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/outbox"
)

func TestDeleter_OldData(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK()
	cluster.Settle(t)

	carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi 1").
		OK().
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi 2").
		OK()
	cluster.Settle(t)

	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello 1").
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello 2").
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello 3").
		OK()

	for _, line := range bob.FollowInput("🔭 View profile", "carol@b.localdomain").Lines {
		if line.Type != gmi.Link {
			continue
		}

		if !strings.HasPrefix(line.URL, "/users/view/") {
			continue
		}

		bob.
			Goto(line.URL).
			Follow("🔁 Share")
	}

	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 2"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 3"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 2"})

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update notes set inserted = inserted - (24 * 60 * 60 * 31) where object->>'$.content' = '<p>hello 1</p>'`); err != nil {
		t.Fatalf("Failed to set post #1 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #1 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #1 insertion time: no rows affected")
	}

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update notes set inserted = inserted - (24 * 60 * 60 * 30) where object->>'$.content' = '<p>hello 2</p>'`); err != nil {
		t.Fatalf("Failed to set post #2 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #2 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #2 insertion time: no rows affected")
	}

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update notes set inserted = inserted - (24 * 60 * 60 * 29) where object->>'$.content' = '<p>hello 3</p>'`); err != nil {
		t.Fatalf("Failed to set post #3 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #3 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #3 insertion time: no rows affected")
	}

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update shares set inserted = inserted - (24 * 60 * 60 * 30) where exists (select 1 from notes where notes.id = shares.note and notes.object->>'$.content' = '<p>hi 2</p>')`); err != nil {
		t.Fatalf("Failed to set share #1 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #1 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #1 insertion time: no rows affected")
	}

	deleter := outbox.Deleter{
		DB:    cluster["b.localdomain"].DB,
		Inbox: cluster["b.localdomain"].Inbox,
	}

	if err := deleter.Run(t.Context()); err != nil {
		t.Fatalf("Deleter has failed: %v", err)
	}

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 2"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 3"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 2"})

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Post deletion policy").
		Contains(gmi.Line{Type: gmi.Text, Text: "Current setting: old posts are not deleted automatically."}).
		Follow("After a month").
		Contains(gmi.Line{Type: gmi.Text, Text: "Current setting: posts are deleted after a month."})

	if err := deleter.Run(t.Context()); err != nil {
		t.Fatalf("Deleter has failed: %v", err)
	}

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello 1"}).
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello 2"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 3"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 1"}).
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hi 2"})

	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello 1"}).
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hello 2"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 3"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 1"}).
		NotContains(gmi.Line{Type: gmi.Quote, Text: "hi 2"})
}

func TestDeleter_Disabled(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["b.localdomain"].Register(carolKeypair).OK()

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Post deletion policy").
		Contains(gmi.Line{Type: gmi.Text, Text: "Current setting: old posts are not deleted automatically."}).
		Follow("After a month").
		Contains(gmi.Line{Type: gmi.Text, Text: "Current setting: posts are deleted after a month."})

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	alice.
		FollowInput("🔭 View profile", "carol@b.localdomain").
		Follow("⚡ Follow carol").
		OK()
	cluster.Settle(t)

	carol.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi 1").
		OK().
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hi 2").
		OK()
	cluster.Settle(t)

	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello 1").
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello 2").
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello 3").
		OK()

	for _, line := range bob.FollowInput("🔭 View profile", "carol@b.localdomain").Lines {
		if line.Type != gmi.Link {
			continue
		}

		if !strings.HasPrefix(line.URL, "/users/view/") {
			continue
		}

		bob.
			Goto(line.URL).
			Follow("🔁 Share")
	}

	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 2"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 3"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 2"})

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update notes set inserted = inserted - (24 * 60 * 60 * 31) where object->>'$.content' = '<p>hello 1</p>'`); err != nil {
		t.Fatalf("Failed to set post #1 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #1 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #1 insertion time: no rows affected")
	}

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update notes set inserted = inserted - (24 * 60 * 60 * 30) where object->>'$.content' = '<p>hello 2</p>'`); err != nil {
		t.Fatalf("Failed to set post #2 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #2 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #2 insertion time: no rows affected")
	}

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update notes set inserted = inserted - (24 * 60 * 60 * 29) where object->>'$.content' = '<p>hello 3</p>'`); err != nil {
		t.Fatalf("Failed to set post #3 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #3 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #3 insertion time: no rows affected")
	}

	if res, err := cluster["b.localdomain"].DB.ExecContext(t.Context(), `update shares set inserted = inserted - (24 * 60 * 60 * 30) where exists (select 1 from notes where notes.id = shares.note and notes.object->>'$.content' = '<p>hi 2</p>')`); err != nil {
		t.Fatalf("Failed to set share #1 insertion time: %v", err)
	} else if n, err := res.RowsAffected(); err != nil {
		t.Fatalf("Failed to set post #1 insertion time: %v", err)
	} else if n == 0 {
		t.Fatal("Failed to set post #1 insertion time: no rows affected")
	}

	bob.
		Follow("⚙️ Settings").
		Follow("⏳ Post deletion policy").
		Contains(gmi.Line{Type: gmi.Text, Text: "Current setting: posts are deleted after a month."}).
		Follow("Never").
		Contains(gmi.Line{Type: gmi.Text, Text: "Current setting: old posts are not deleted automatically."})

	deleter := outbox.Deleter{
		DB:    cluster["b.localdomain"].DB,
		Inbox: cluster["b.localdomain"].Inbox,
	}

	if err := deleter.Run(t.Context()); err != nil {
		t.Fatalf("Deleter has failed: %v", err)
	}

	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 2"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 3"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 2"})

	cluster.Settle(t)

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 2"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hello 3"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 1"}).
		Contains(gmi.Line{Type: gmi.Quote, Text: "hi 2"})
}
