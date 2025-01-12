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

package cluster

import (
	"context"
	"testing"

	"github.com/dimkr/tootik/fed"
)

func TestFederation_FollowersSyncMissingRemoteFollow(t *testing.T) {
	f := NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["b.localdomain"].Register(bobKeypair).OK()
	carol := f["a.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	// this second follow is required so posts by bob get sent to a.localdomain although b.localdomain thinks alice is not following
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	f.Settle()

	// delete the Follow in b.localdomain
	if _, err := f["b.localdomain"].DB.Exec(`delete from follows where follower = 'https://a.localdomain/user/alice'`); err != nil {
		t.Fatalf("Failed to delete follow: %v", err)
	}

	// this causes Collection-Synchronization to be sent from b.localdomain to a.localdomain
	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	f.Settle()

	var exists int
	if err := f["a.localdomain"].DB.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted = 1)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow exists: %v", err)
	}
	if exists == 0 {
		t.Fatal("Follow does not exist")
	}

	syncer := fed.Syncer{
		Domain:   "a.localdomain",
		Config:   f["a.localdomain"].Config,
		DB:       f["a.localdomain"].DB,
		Resolver: f["a.localdomain"].Resolver,
		Key:      f["a.localdomain"].NobodyKey,
	}
	if _, err := syncer.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to synchronize followers: %v", err)
	}
	f.Settle()

	if err := f["a.localdomain"].DB.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted = 1)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow was removed: %v", err)
	}
	if exists == 1 {
		t.Fatal("Follow was not removed")
	}
}

func TestFederation_FollowersSyncMissingLocalFollow(t *testing.T) {
	f := NewFediverse(t, "a.localdomain", "b.localdomain")
	defer f.Stop()

	alice := f["a.localdomain"].Register(aliceKeypair).OK()
	bob := f["b.localdomain"].Register(bobKeypair).OK()
	carol := f["a.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	// this second follow is required so posts by bob get sent to a.localdomain although b.localdomain thinks alice is not following
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	f.Settle()

	// delete one Follow in a.localdomain
	if _, err := f["a.localdomain"].DB.Exec(`delete from follows where follower = 'https://a.localdomain/user/alice'`); err != nil {
		t.Fatalf("Failed to delete follow: %v", err)
	}

	// this causes Collection-Synchronization to be sent from b.localdomain to a.localdomain
	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	f.Settle()

	var exists int
	if err := f["b.localdomain"].DB.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted = 1)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow exists: %v", err)
	}
	if exists == 0 {
		t.Fatal("Follow does not exist")
	}

	syncer := fed.Syncer{
		Domain:   "a.localdomain",
		Config:   f["a.localdomain"].Config,
		DB:       f["a.localdomain"].DB,
		Resolver: f["a.localdomain"].Resolver,
		Key:      f["a.localdomain"].NobodyKey,
	}
	if _, err := syncer.ProcessBatch(context.Background()); err != nil {
		t.Fatalf("Failed to synchronize followers: %v", err)
	}
	f.Settle()

	if err := f["b.localdomain"].DB.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted = 1)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow was removed: %v", err)
	}
	if exists == 1 {
		t.Fatal("Follow was not removed")
	}
}
