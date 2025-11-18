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

func TestCluster_Invite(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	cluster["a.localdomain"].Config.RequireInvite = true

	invites := alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Create").
		Follow("➕ Create")

	for _, line := range invites.Lines {
		if line.Type != Text {
			continue
		}

		if id, ok := strings.CutPrefix(line.Text, "ID: "); ok {
			cluster["a.localdomain"].HandleInput(bobKeypair, "/users/invites/accept", id).OK()
		}
	}
}
