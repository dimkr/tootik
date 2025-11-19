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

func TestCluster_InvitationHappyFlow(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	cluster["a.localdomain"].Config.RequireInvite = true

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"
	carolCode := "ded3626c-ea4b-44cc-adf3-18510e7634e1"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		FollowInput("➕ Create", bobCode).
		Contains(Line{Type: Text, Text: "Code: " + bobCode})

	cluster["a.localdomain"].HandleInput(bobKeypair, "/users/invitations/accept", bobCode).Follow("😈 My profile").OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Contains(Line{Type: Link, Text: "Used by: bob", URL: "/users/outbox/a.localdomain/user/bob"}).
		FollowInput("➕ Create", carolCode).
		Contains(Line{Type: Text, Text: "Code: " + carolCode})

	cluster["a.localdomain"].HandleInput(carolKeypair, "/users/invitations/accept", carolCode).Follow("😈 My profile").OK()
}

func TestCluster_WrongCode(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	cluster["a.localdomain"].Config.RequireInvite = true

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"
	carolCode := "ded3626c-ea4b-44cc-adf3-18510e7634e1"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		FollowInput("➕ Create", bobCode).
		Contains(Line{Type: Text, Text: "Code: " + bobCode})

	cluster["a.localdomain"].HandleInput(bobKeypair, "/users/invitations/accept", carolCode).Error("40 Invalid invitation code")

	cluster["a.localdomain"].HandleInput(bobKeypair, "/users/invitations/accept", bobCode).Follow("😈 My profile").OK()
}

func TestCluster_CodeReuse(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	cluster["a.localdomain"].Config.RequireInvite = true

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		FollowInput("➕ Create", bobCode).
		Contains(Line{Type: Text, Text: "Code: " + bobCode})

	cluster["a.localdomain"].HandleInput(bobKeypair, "/users/invitations/accept", bobCode).Follow("😈 My profile").OK()

	cluster["a.localdomain"].HandleInput(bobKeypair, "/users/invitations/accept", bobCode).Error("40 Invalid invitation code")
	cluster["a.localdomain"].HandleInput(carolKeypair, "/users/invitations/accept", bobCode).Error("40 Invalid invitation code")
}

func TestCluster_InvitationLimit(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	cluster["a.localdomain"].Config.RequireInvite = true
	limit := 1
	cluster["a.localdomain"].Config.MaxInvitesPerUser = &limit

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"
	carolCode := "ded3626c-ea4b-44cc-adf3-18510e7634e1"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Contains(Line{Type: Link, Text: "➕ Create", URL: "/users/invitations/create"}).
		NotContains(Line{Type: Text, Text: "Reached the maximum number of invitations."}).
		FollowInput("➕ Create", bobCode).
		Contains(Line{Type: Text, Text: "Code: " + bobCode}).
		NotContains(Line{Type: Link, Text: "➕ Create", URL: "/users/invitations/create"}).
		Contains(Line{Type: Text, Text: "Reached the maximum number of invitations."})

	alice.Goto("/users/invitations/create").
		Error("40 Reached the maximum number of invitations")

	cluster["a.localdomain"].HandleInput(bobKeypair, "/users/invitations/accept", bobCode).Follow("😈 My profile").OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Contains(Line{Type: Link, Text: "Used by: bob", URL: "/users/outbox/a.localdomain/user/bob"}).
		FollowInput("➕ Create", carolCode).
		Contains(Line{Type: Text, Text: "Code: " + carolCode}).
		NotContains(Line{Type: Link, Text: "➕ Create", URL: "/users/invitations/create"}).
		Contains(Line{Type: Text, Text: "Reached the maximum number of invitations."})

	cluster["a.localdomain"].HandleInput(carolKeypair, "/users/invitations/accept", carolCode).Follow("😈 My profile").OK()

	limit = 3
	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Create").
		Follow("➕ Create").
		Follow("➕ Create").
		NotContains(Line{Type: Link, Text: "➕ Create", URL: "/users/invitations/create"}).
		Contains(Line{Type: Text, Text: "Reached the maximum number of invitations."})

	alice.Goto("/users/invitations/create").
		Error("40 Reached the maximum number of invitations")

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➖ Delete").
		NotContains(Line{Type: Text, Text: "Reached the maximum number of invitations."}).
		Follow("➕ Create").
		Contains(Line{Type: Text, Text: "Reached the maximum number of invitations."})
}

func TestCluster_InvitationCreateDeleteAccept(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	cluster["a.localdomain"].Config.RequireInvite = true

	page := alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Create")

	var code string
	found := false
	for _, line := range page.Lines {
		if line.Type != Text {
			continue
		}

		if code, found = strings.CutPrefix(line.Text, "Code: "); found {
			break
		}
	}

	if !found {
		t.Fatalf("Not found")
	}

	page.
		Contains(Line{Type: Text, Text: "Code: " + code}).
		Follow("➖ Delete").
		NotContains(Line{Type: Text, Text: "Code: " + code})

	cluster["a.localdomain"].
		HandleInput(bobKeypair, "/users/invitations/accept", code).
		Error("40 Invalid invitation code")

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		NotContains(Line{Type: Text, Text: "Code: " + code})
}

func TestCluster_InvitationCreateAcceptDelete(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain", "c.localdomain")
	defer cluster.Stop()

	alice := cluster["a.localdomain"].Register(aliceKeypair).OK()

	cluster["a.localdomain"].Config.RequireInvite = true

	page := alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Create")

	var code string
	found := false
	for _, line := range page.Lines {
		if line.Type != Text {
			continue
		}

		if code, found = strings.CutPrefix(line.Text, "Code: "); found {
			break
		}
	}

	if !found {
		t.Fatalf("Not found")
	}

	cluster["a.localdomain"].
		HandleInput(bobKeypair, "/users/invitations/accept", code).
		OK()

	page.
		Follow("➖ Delete").
		Error("40 Invalid invitation code")
}
