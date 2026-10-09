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

package ap

import "testing"

const otherTestKey = "z6MksgCbQa3BZxBayRRkF1hcP7zt6TZGvZF2rR1k3AY7zFL8"

func TestValidateOrigin_MixedIDs(t *testing.T) {
	did := "did:key:" + testKey
	canonicalActor := "ap://" + did + "/actor"
	hintedActor := canonicalActor + "?@gateway=https%3A%2F%2Fa.example"
	encodedActor := "ap+ef61://did%3Akey%3A" + testKey + "/actor"
	compatibleActor := "https://a.example/.well-known/apgateway/" + did + "/actor"
	foreignGatewayActor := "https://b.example/.well-known/apgateway/" + did + "/actor"
	otherActor := "ap://did:key:" + otherTestKey + "/actor"

	compatiblePost := "https://a.example/.well-known/apgateway/" + did + "/actor/post/1"
	canonicalPost := "ap://" + did + "/actor/post/1"

	for _, tc := range []struct {
		name     string
		activity Activity
		valid    bool
	}{
		{
			"compatible post by ap actor",
			Activity{ID: canonicalActor + "/create/1", Type: Create, Actor: canonicalActor, Object: &Object{ID: compatiblePost, AttributedTo: canonicalActor}},
			true,
		},
		{
			"ap post by compatible actor",
			Activity{ID: compatibleActor + "/create/1", Type: Create, Actor: compatibleActor, Object: &Object{ID: canonicalPost, AttributedTo: compatibleActor}},
			true,
		},
		{
			"author with hints",
			Activity{ID: canonicalActor + "/create/1", Type: Create, Actor: canonicalActor, Object: &Object{ID: canonicalPost, AttributedTo: hintedActor}},
			true,
		},
		{
			"ap+ef61 actor with encoded authority",
			Activity{ID: canonicalActor + "/update/1", Type: Update, Actor: encodedActor, Object: &Object{ID: compatiblePost, AttributedTo: canonicalActor}},
			true,
		},
		{
			"foreign gateway for the same DID",
			Activity{ID: foreignGatewayActor + "/create/1", Type: Create, Actor: foreignGatewayActor, Object: &Object{ID: compatiblePost, AttributedTo: hintedActor}},
			true,
		},
		{
			"post by a different DID",
			Activity{ID: canonicalActor + "/create/1", Type: Create, Actor: canonicalActor, Object: &Object{ID: compatiblePost, AttributedTo: otherActor}},
			false,
		},
		{
			"post owned by a different DID",
			Activity{ID: canonicalActor + "/create/1", Type: Create, Actor: canonicalActor, Object: &Object{ID: "ap://did:key:" + otherTestKey + "/post/1", AttributedTo: canonicalActor}},
			false,
		},
		{
			"actor with a different DID",
			Activity{ID: canonicalActor + "/create/1", Type: Create, Actor: otherActor, Object: &Object{ID: canonicalPost, AttributedTo: otherActor}},
			false,
		},
		{
			"delete compatible post by ap actor",
			Activity{ID: canonicalActor + "/delete/1", Type: Delete, Actor: hintedActor, Object: compatiblePost},
			true,
		},
		{
			"delete post of a different DID",
			Activity{ID: canonicalActor + "/delete/1", Type: Delete, Actor: canonicalActor, Object: "ap://did:key:" + otherTestKey + "/post/1"},
			false,
		},
		{
			"undo follow across forms",
			Activity{ID: canonicalActor + "/undo/1", Type: Undo, Actor: canonicalActor, Object: &Activity{ID: compatibleActor + "/follow/1", Type: Follow, Actor: compatibleActor, Object: "https://localhost.localdomain/user/bob"}},
			true,
		},
		{
			"undo follow by a different DID",
			Activity{ID: canonicalActor + "/undo/1", Type: Undo, Actor: canonicalActor, Object: &Activity{ID: otherActor + "/follow/1", Type: Follow, Actor: otherActor, Object: "https://localhost.localdomain/user/bob"}},
			false,
		},
		{
			"quote request across forms",
			Activity{ID: canonicalActor + "/quote/1", Type: QuoteRequest, Actor: canonicalActor, Instrument: &Object{ID: compatiblePost, AttributedTo: hintedActor}, Object: "https://localhost.localdomain/post/1"},
			true,
		},
		{
			"quote request by a different DID",
			Activity{ID: canonicalActor + "/quote/1", Type: QuoteRequest, Actor: canonicalActor, Instrument: &Object{ID: compatiblePost, AttributedTo: otherActor}, Object: "https://localhost.localdomain/post/1"},
			false,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			origin, err := Origin(tc.activity.ID)
			if err != nil {
				t.Fatalf("Failed to get origin of %s: %v", tc.activity.ID, err)
			}

			if err := ValidateOrigin("localhost.localdomain", &tc.activity, origin); tc.valid && err != nil {
				t.Fatalf("Valid activity is invalid: %v", err)
			} else if !tc.valid && err == nil {
				t.Fatal("Invalid activity is valid")
			}
		})
	}
}
