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
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func postToSharedInbox(t *testing.T, server *Server, activity *ap.Activity) int {
	j, err := json.Marshal(activity)
	if err != nil {
		t.Fatalf("Failed to marshal activity: %v", err)
	}

	r, err := http.NewRequestWithContext(t.Context(), http.MethodPost, "https://"+server.Domain+"/inbox", bytes.NewReader(j))
	if err != nil {
		t.Fatalf("Failed to create HTTP request: %v", err)
	}

	w := responseWriter{Headers: http.Header{}}
	server.Backend.ServeHTTP(&w, r)
	return w.StatusCode
}

func TestCluster_CanonicalIDsMixedPost(t *testing.T) {
	cluster := NewCluster(t, "a.localdomain", "b.localdomain")
	defer cluster.Stop()

	cluster["a.localdomain"].Config.CanonicalIDs = true

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	_, otherPriv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	did := "did:key:" + data.EncodeEd25519PublicKey(pub)
	actorID := "ap://" + did + "/actor"
	hintedActorID := actorID + "?@gateway=https%3A%2F%2Fa.localdomain"
	compatibleActorID := "https://a.localdomain/.well-known/apgateway/" + did + "/actor"

	cluster["a.localdomain"].Handle(aliceKeypair, "/users/register?"+data.EncodeEd25519PrivateKey(priv)).OK()
	bob := cluster["b.localdomain"].RegisterPortable(bobKeypair).OK()

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Follow("⚡ Follow alice").
		OK()
	cluster.Settle(t)

	to := ap.Audience{}
	to.Add(ap.Public)

	create := func(n string, signer ed25519.PrivateKey, withProof bool) *ap.Activity {
		create := &ap.Activity{
			Context:   []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v1"},
			Type:      ap.Create,
			ID:        actorID + "/create/" + n,
			Actor:     hintedActorID,
			To:        to,
			CC:        to,
			Published: ap.Time{Time: time.Now()},
			Object: &ap.Object{
				Type:         ap.Note,
				ID:           compatibleActorID + "/post/" + n,
				Content:      "post " + n,
				AttributedTo: actorID,
				To:           to,
				CC:           to,
				Published:    ap.Time{Time: time.Now()},
			},
		}

		if !withProof {
			return create
		}

		var err error
		if create.Proof, err = proof.Create(httpsig.Key{ID: actorID + "#ed25519-key", PrivateKey: signer}, create); err != nil {
			t.Fatalf("Failed to generate proof: %v", err)
		}

		return create
	}

	if status := postToSharedInbox(t, cluster["b.localdomain"], create("1", priv, true)); status != http.StatusAccepted {
		t.Fatalf("Valid activity was rejected: %d", status)
	}

	if status := postToSharedInbox(t, cluster["b.localdomain"], create("2", otherPriv, true)); status != http.StatusUnauthorized {
		t.Fatalf("Activity with a proof by another key was accepted: %d", status)
	}

	if status := postToSharedInbox(t, cluster["b.localdomain"], create("3", nil, false)); status != http.StatusUnauthorized {
		t.Fatalf("Activity without a proof was accepted: %d", status)
	}

	cluster.Settle(t)

	bob.
		FollowInput("🔭 View profile", "alice@a.localdomain").
		Contains(gmi.Line{Type: gmi.Quote, Text: "post 1"}).
		NotContains(gmi.Line{Type: gmi.Quote, Text: "post 2"}).
		NotContains(gmi.Line{Type: gmi.Quote, Text: "post 3"})
}
