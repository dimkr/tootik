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
	"time"

	"github.com/dimkr/tootik/gemtext"
)

func TestServer_InvitationHappyFlow(t *testing.T) {
	s := NewServer(t, "a.localdomain", Client{})

	alice := s.Register(aliceKeypair).OK()

	s.Config.RequireInvitation = true
	s.Config.EnableNonPortableActorRegistration = true

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"
	carolCode := "ded3626c-ea4b-44cc-adf3-18510e7634e1"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		FollowInput("➕ Generate", bobCode).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + bobCode})

	accept := s.HandleInput(bobKeypair, "/users/invitations/accept", bobCode)
	accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
	s.HandleInput(bobKeypair, accept.Path, "n").Follow("😈 My profile").OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Contains(gemtext.Line{Type: gemtext.Link, Text: "Used by: bob", URL: "/users/outbox/a.localdomain/user/bob"}).
		FollowInput("➕ Generate", carolCode).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + carolCode})

	accept = s.HandleInput(carolKeypair, "/users/invitations/accept", carolCode)
	accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
	s.HandleInput(carolKeypair, accept.Path, "generate").Follow("😈 My profile").OK()
}

func TestServer_WrongCode(t *testing.T) {
	s := NewServer(t, "a.localdomain", Client{})

	alice := s.Register(aliceKeypair).OK()

	s.Config.RequireInvitation = true
	s.Config.EnableNonPortableActorRegistration = true

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"
	carolCode := "ded3626c-ea4b-44cc-adf3-18510e7634e1"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		FollowInput("➕ Generate", bobCode).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + bobCode})

	s.HandleInput(bobKeypair, "/users/invitations/accept", carolCode).Error("40 Invalid invitation code")

	accept := s.HandleInput(bobKeypair, "/users/invitations/accept", bobCode)
	accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
	s.HandleInput(bobKeypair, accept.Path, "generate").Follow("😈 My profile").OK()
}

func TestServer_ExpiredCode(t *testing.T) {
	s := NewServer(t, "a.localdomain", Client{})

	alice := s.Register(aliceKeypair).OK()

	s.Config.RequireInvitation = true
	s.Config.InvitationTimeout = 1

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		FollowInput("➕ Generate", bobCode).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + bobCode})

	select {
	case <-time.After(1):
		s.HandleInput(bobKeypair, "/users/invitations/accept", bobCode).Error("40 Invalid invitation code")

		s.Config.InvitationTimeout = time.Hour

		accept := s.HandleInput(bobKeypair, "/users/invitations/accept", bobCode)
		accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
		s.HandleInput(bobKeypair, accept.Path, "generate").Follow("😈 My profile").OK()

	case <-t.Context().Done():
		t.Fail()
	}
}

func TestServer_CodeReuse(t *testing.T) {
	s := NewServer(t, "a.localdomain", Client{})

	alice := s.Register(aliceKeypair).OK()

	s.Config.RequireInvitation = true

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		FollowInput("➕ Generate", bobCode).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + bobCode})

	accept := s.HandleInput(bobKeypair, "/users/invitations/accept", bobCode)
	accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
	s.HandleInput(bobKeypair, accept.Path, "generate").Follow("😈 My profile").OK()

	s.HandleInput(bobKeypair, "/users/invitations/accept", bobCode).Error("40 Invalid invitation code")
	s.HandleInput(carolKeypair, "/users/invitations/accept", bobCode).Error("40 Invalid invitation code")
}

func TestServer_InvitationLimit(t *testing.T) {
	s := NewServer(t, "a.localdomain", Client{})

	alice := s.Register(aliceKeypair).OK()

	s.Config.RequireInvitation = true
	s.Config.MaxInvitationsPerUser = new(1)
	s.Config.EnableNonPortableActorRegistration = true

	bobCode := "70bc9fdf-74a4-41e5-973d-08ba3fd23d74"
	carolCode := "ded3626c-ea4b-44cc-adf3-18510e7634e1"

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Contains(gemtext.Line{Type: gemtext.Link, Text: "➕ Generate", URL: "/users/invitations/generate"}).
		NotContains(gemtext.Line{Type: gemtext.Text, Text: "Reached the maximum number of invitations."}).
		FollowInput("➕ Generate", bobCode).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + bobCode}).
		NotContains(gemtext.Line{Type: gemtext.Link, Text: "➕ Generate", URL: "/users/invitations/generate"}).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Reached the maximum number of invitations."})

	alice.Goto("/users/invitations/generate").
		Error("40 Reached the maximum number of invitations")

	accept := s.HandleInput(bobKeypair, "/users/invitations/accept", bobCode)
	accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
	s.HandleInput(bobKeypair, accept.Path, "n").Follow("😈 My profile").OK()

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Contains(gemtext.Line{Type: gemtext.Link, Text: "Used by: bob", URL: "/users/outbox/a.localdomain/user/bob"}).
		FollowInput("➕ Generate", carolCode).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + carolCode}).
		NotContains(gemtext.Line{Type: gemtext.Link, Text: "➕ Generate", URL: "/users/invitations/generate"}).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Reached the maximum number of invitations."})

	accept = s.HandleInput(carolKeypair, "/users/invitations/accept", carolCode)
	accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
	s.HandleInput(carolKeypair, accept.Path, "generate").Follow("😈 My profile").OK()

	s.Config.MaxInvitationsPerUser = new(3)
	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Generate").
		Follow("➕ Generate").
		Follow("➕ Generate").
		NotContains(gemtext.Line{Type: gemtext.Link, Text: "➕ Generate", URL: "/users/invitations/generate"}).
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Reached the maximum number of invitations."})

	alice.Goto("/users/invitations/generate").
		Error("40 Reached the maximum number of invitations")

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➖ Revoke").
		NotContains(gemtext.Line{Type: gemtext.Text, Text: "Reached the maximum number of invitations."}).
		Follow("➕ Generate").
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Reached the maximum number of invitations."})
}

func TestServer_InvitationCreateDeleteAccept(t *testing.T) {
	s := NewServer(t, "a.localdomain", Client{})

	alice := s.Register(aliceKeypair).OK()

	s.Config.RequireInvitation = true

	page := alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Generate")

	var code string
	found := false
	for _, line := range page.Lines {
		if line.Type != gemtext.Text {
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
		Contains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + code}).
		Follow("➖ Revoke").
		NotContains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + code})

	s.
		HandleInput(bobKeypair, "/users/invitations/accept", code).
		Error("40 Invalid invitation code")

	alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		NotContains(gemtext.Line{Type: gemtext.Text, Text: "Code: " + code})
}

func TestServer_InvitationCreateAcceptDelete(t *testing.T) {
	s := NewServer(t, "a.localdomain", Client{})

	alice := s.Register(aliceKeypair).OK()

	s.Config.RequireInvitation = true

	page := alice.
		Follow("⚙️ Settings").
		Follow("🎟️ Invitations").
		Follow("➕ Generate")

	var code string
	found := false
	for _, line := range page.Lines {
		if line.Type != gemtext.Text {
			continue
		}

		if code, found = strings.CutPrefix(line.Text, "Code: "); found {
			break
		}
	}

	if !found {
		t.Fatalf("Not found")
	}

	accept := s.HandleInput(bobKeypair, "/users/invitations/accept", code)
	accept.Error("11 base58-encoded Ed25519 private key or 'generate' to generate")
	s.HandleInput(bobKeypair, accept.Path, "generate").OK()

	page.
		Follow("➖ Revoke").
		Error("40 Invalid invitation code")
}
