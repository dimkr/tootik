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

import "testing"

func TestCluster_Invite(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	cluster["a.localdomain"].Config.EnablePortableActorRegistration = true
	cluster["b.localdomain"].Config.EnablePortableActorRegistration = true
	cluster["c.localdomain"].Config.EnablePortableActorRegistration = true

	alice := cluster["a.localdomain"].RegisterPortable(aliceKeypair).OK()
	bob := cluster["b.localdomain"].RegisterPortable(bobKeypair).OK()
	carol := cluster["c.localdomain"].RegisterPortable(carolKeypair).OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Invite by newly generated key").
		OK()
	cluster.Settle(t)

	_ = bob
	_ = carol
}
