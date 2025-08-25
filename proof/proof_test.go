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

// https://codeberg.org/fediverse/fep/src/commit/3a5942066f989d8317befe6457b48237bc61efe0/fep/8b32/fep-8b32.feature#L3
func TestProof_Sign(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"id":"https://server.example/activities/1","type":"Create","actor":"https://server.example/users/alice","object":{"id":"https://server.example/objects/1","type":"Note","attributedTo":"https://server.example/users/alice","content":"Hello world","location":{"type":"Place","longitude":-71.184902,"latitude":25.273962}}}`)

	var a ap.Activity
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	}

	created, err := time.Parse(time.RFC3339, "2023-02-24T23:36:38Z")
	if err != nil {
		t.Fatalf("Failed to parse creation timestamp: %v", err)
	}

	privKey := ed25519.NewKeyFromSeed(base58.Decode("3u2en7t5LR2WtQH5PfFqMqwVHBeXouLzo6haApm8XHqvjxq")[2:])

	if withProof, err := Add(httpsig.Key{ID: "https://server.example/users/alice#ed25519-key", PrivateKey: privKey}, created, raw); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	} else if err := json.Unmarshal(withProof, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	} else if a.Proof.Value != "zLaewdp4H9kqtwyrLatK4cjY5oRHwVcw4gibPSUDYDMhi4M49v8pcYk3ZB6D69dNpAPbUmY8ocuJ3m9KhKJEEg7z" {
		t.Fatalf("Unexpected proof value: %s", a.Proof.Value)
	} else if err := Verify(privKey.Public(), a.Proof, withProof); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
}

// https://codeberg.org/fediverse/fep/src/commit/3a5942066f989d8317befe6457b48237bc61efe0/fep/8b32/fep-8b32.feature#L67
func TestProof_Verify(t *testing.T) {
	t.Parallel()

	raw := []byte(`{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"id":"https://server.example/activities/1","type":"Create","actor":"https://server.example/users/alice","object":{"id":"https://server.example/objects/1","type":"Note","attributedTo":"https://server.example/users/alice","content":"Hello world","location":{"type":"Place","longitude":-71.184902,"latitude":25.273962}},"proof":{"@context":["https://www.w3.org/ns/activitystreams","https://w3id.org/security/data-integrity/v1"],"type":"DataIntegrityProof","cryptosuite":"eddsa-jcs-2022","verificationMethod":"https://server.example/users/alice#ed25519-key","proofPurpose":"assertionMethod","proofValue":"zLaewdp4H9kqtwyrLatK4cjY5oRHwVcw4gibPSUDYDMhi4M49v8pcYk3ZB6D69dNpAPbUmY8ocuJ3m9KhKJEEg7z","created":"2023-02-24T23:36:38Z"}}`)

	var a ap.Activity
	if err := json.Unmarshal(raw, &a); err != nil {
		t.Fatalf("Failed to unmarshal activity: %v", err)
	}

	if err := Verify(ed25519.PublicKey(base58.Decode("6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2")[2:]), a.Proof, raw); err != nil {
		t.Fatalf("Failed to verify proof: %v", err)
	}
}
