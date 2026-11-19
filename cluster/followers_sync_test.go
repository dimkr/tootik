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
	"crypto/ed25519"
	"testing"

	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
)

func TestCluster_FollowersSyncMissingRemoteFollow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	// this second follow is required so posts by bob get sent to a.localdomain although b.localdomain thinks alice is not following
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	// delete the Follow in b.localdomain
	if _, err := cluster["b.localdomain"].DBW.Exec(`delete from follows where follower = 'https://a.localdomain/user/alice'`); err != nil {
		t.Fatalf("Failed to delete follow: %v", err)
	}

	// this causes Collection-Synchronization to be sent from b.localdomain to a.localdomain
	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	var exists bool
	if err := cluster["a.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow exists: %v", err)
	}
	if !exists {
		t.Fatal("Follow does not exist")
	}

	syncer := fed.Syncer{
		Domain:   "a.localdomain",
		Config:   cluster["a.localdomain"].Config,
		DBW:      cluster["a.localdomain"].DBW,
		Resolver: cluster["a.localdomain"].Resolver,
		Keys:     cluster["a.localdomain"].AppActorKeys,
		Inbox:    cluster["a.localdomain"].Inbox,
	}
	if _, err := syncer.ProcessBatch(t.Context()); err != nil {
		t.Fatalf("Failed to synchronize followers: %v", err)
	}
	cluster.Settle(t)

	if err := cluster["a.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow was removed: %v", err)
	}
	if exists {
		t.Fatal("Follow was not removed")
	}
}

func TestCluster_FollowersSyncMissingLocalFollow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Register(bobKeypair).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	// this second follow is required so posts by bob get sent to a.localdomain although b.localdomain thinks alice is not following
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	// delete one Follow in a.localdomain
	if _, err := cluster["a.localdomain"].DBW.Exec(`delete from follows where follower = 'https://a.localdomain/user/alice'`); err != nil {
		t.Fatalf("Failed to delete follow: %v", err)
	}

	// this causes Collection-Synchronization to be sent from b.localdomain to a.localdomain
	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	var exists bool
	if err := cluster["b.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow exists: %v", err)
	}
	if !exists {
		t.Fatal("Follow does not exist")
	}

	syncer := fed.Syncer{
		Domain:   "a.localdomain",
		Config:   cluster["a.localdomain"].Config,
		DBW:      cluster["a.localdomain"].DBW,
		Resolver: cluster["a.localdomain"].Resolver,
		Keys:     cluster["a.localdomain"].AppActorKeys,
		Inbox:    cluster["a.localdomain"].Inbox,
	}
	if _, err := syncer.ProcessBatch(t.Context()); err != nil {
		t.Fatalf("Failed to synchronize followers: %v", err)
	}
	cluster.Settle(t)

	if err := cluster["b.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/user/bob' and accepted)`).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow was removed: %v", err)
	}
	if exists {
		t.Fatal("Follow was not removed")
	}
}

func TestCluster_FollowersSyncMissingRemoteFollowPortableActor(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerBob := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	bobDID := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Handle(bobKeypair, registerBob).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	// this second follow is required so posts by bob get sent to a.localdomain although b.localdomain thinks alice is not following
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	// delete the Follow in b.localdomain
	if _, err := cluster["b.localdomain"].DBW.Exec(`delete from follows where follower = 'https://a.localdomain/user/alice'`); err != nil {
		t.Fatalf("Failed to delete follow: %v", err)
	}

	// this causes Collection-Synchronization to be sent from b.localdomain to a.localdomain
	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	var exists bool
	if err := cluster["a.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/.well-known/apgateway/' || $1 || '/actor' and accepted)`, bobDID).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow exists: %v", err)
	}
	if !exists {
		t.Fatal("Follow does not exist")
	}

	syncer := fed.Syncer{
		Domain:   "a.localdomain",
		Config:   cluster["a.localdomain"].Config,
		DBW:      cluster["a.localdomain"].DBW,
		Resolver: cluster["a.localdomain"].Resolver,
		Keys:     cluster["a.localdomain"].AppActorKeys,
		Inbox:    cluster["a.localdomain"].Inbox,
	}
	if _, err := syncer.ProcessBatch(t.Context()); err != nil {
		t.Fatalf("Failed to synchronize followers: %v", err)
	}
	cluster.Settle(t)

	if err := cluster["a.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/.well-known/apgateway/' || $1 || '/actor' and accepted)`, bobDID).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow was removed: %v", err)
	}
	if exists {
		t.Fatal("Follow was not removed")
	}
}

func TestCluster_FollowersSyncMissingLocalFollowPortableActor(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}
	registerBob := "/users/register?" + data.EncodeEd25519PrivateKey(priv)

	bobDID := "did:key:" + data.EncodeEd25519PublicKey(pub)

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()
	bob := cluster["b.localdomain"].Handle(bobKeypair, registerBob).OK()
	carol := cluster["a.localdomain"].Register(carolKeypair).OK()

	alice.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	// this second follow is required so posts by bob get sent to a.localdomain although b.localdomain thinks alice is not following
	carol.
		FollowInput("🔭 View profile", "bob@b.localdomain").
		Follow("⚡ Follow bob").
		OK()
	cluster.Settle(t)

	// delete one Follow in a.localdomain
	if _, err := cluster["a.localdomain"].DBW.Exec(`delete from follows where follower = 'https://a.localdomain/user/alice'`); err != nil {
		t.Fatalf("Failed to delete follow: %v", err)
	}

	// this causes Collection-Synchronization to be sent from b.localdomain to a.localdomain
	bob.
		Follow("📣 New post").
		FollowInput("📣 Anyone", "hello").
		OK()
	cluster.Settle(t)

	var exists bool
	if err := cluster["b.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/.well-known/apgateway/' || $1 || '/actor' and accepted)`, bobDID).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow exists: %v", err)
	}
	if !exists {
		t.Fatal("Follow does not exist")
	}

	syncer := fed.Syncer{
		Domain:   "a.localdomain",
		Config:   cluster["a.localdomain"].Config,
		DBW:      cluster["a.localdomain"].DBW,
		Resolver: cluster["a.localdomain"].Resolver,
		Keys:     cluster["a.localdomain"].AppActorKeys,
		Inbox:    cluster["a.localdomain"].Inbox,
	}
	if _, err := syncer.ProcessBatch(t.Context()); err != nil {
		t.Fatalf("Failed to synchronize followers: %v", err)
	}
	cluster.Settle(t)

	if err := cluster["b.localdomain"].DBW.QueryRow(`select exists (select 1 from follows where follower = 'https://a.localdomain/user/alice' and followed = 'https://b.localdomain/.well-known/apgateway/' || $1 || '/actor' and accepted)`, bobDID).Scan(&exists); err != nil {
		t.Fatalf("Failed to check if follow was removed: %v", err)
	}
	if exists {
		t.Fatal("Follow was not removed")
	}
}
