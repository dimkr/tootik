/*
Copyright 2025 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless ruired by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package proof

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

func TestProof(t *testing.T) {
	t.Parallel()

	to := ap.Audience{}
	to.Add("https://b.localdomain/user/bob")
	to.Add("https://www.w3.org/ns/activitystreams#Public")

	cc := ap.Audience{}
	cc.Add("https://a.localdomain/followers/alice")
	cc.Add("https://b.localdomain/user/bob")

	a := ap.Activity{
		Context: "https://www.w3.org/ns/activitystreams",
		ID:      "https://a.localdomain/create/78625046-8744-47f1-9d5b-f5e6b503e14c",
		Type:    ap.Create,
		Actor:   "https://a.localdomain/user/alice",
		Object: ap.Object{
			ID:           "https://a.localdomain/post/a1ff631e-e658-4623-8c0f-d71c3d881913",
			Type:         ap.Note,
			AttributedTo: "https://a.localdomain/user/alice",
			InReplyTo:    "https://b.localdomain/post/8f8c892e-1442-4cb2-8b7b-cf9c5b50f951",
			Content:      "<p><span class=\"h-card\" translate=\"no\"><a href=\"https://b.localdomain/user/bob\" class=\"u-url mention\">@bob</a></span> No</p>",
			Published:    ap.Time{Time: time.Now()},
			To:           to,
			CC:           cc,
			Tag: []ap.Tag{
				{
					Type: ap.Mention,
					Name: "@bob",
					Href: "https://b.localdomain/user/bob",
				},
			},
		},
		To: to,
		CC: cc,
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	proof, err := Create(httpsig.Key{ID: "abcd", PrivateKey: priv}, time.Now(), &a)
	if err != nil {
		t.Fatalf("Failed to create proof: %v", err)
	}

	a.Proof = proof

	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Failed to marshal activity with proof: %v", err)
	}

	if err := Verify(pub, &a, raw); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
}

// https://codeberg.org/fediverse/fep/src/commit/3a5942066f989d8317befe6457b48237bc61efe0/fep/8b32/fep-8b32.feature#L67
func TestProof_Vector(t *testing.T) {
	raw := []byte(`{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"id":"https://server.example/activities/1","type":"Create","actor":"https://server.example/users/alice","object":{"id":"https://server.example/objects/1","type":"Note","attributedTo":"https://server.example/users/alice","content":"Hello world","location":{"type":"Place","longitude":-71.184902,"latitude":25.273962}},"proof":{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"type":"DataIntegrityProof","cryptosuite":"eddsa-jcs-2022","verificationMethod":"https://server.example/users/alice#ed25519-key","proofPurpose":"assertionMethod","proofValue":"zLaewdp4H9kqtwyrLatK4cjY5oRHwVcw4gibPSUDYDMhi4M49v8pcYk3ZB6D69dNpAPbUmY8ocuJ3m9KhKJEEg7z","created":"2023-02-24T23:36:38Z"}}`)

	var a ap.Activity
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	}

	if err := Verify(ed25519.PublicKey(base58.Decode("6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2")[2:]), &a, raw); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
}
