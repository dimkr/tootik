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

package ap

import (
	"slices"
	"testing"
)

// https://codeberg.org/fediverse/fep/src/commit/480415584237eb19cb7373b6a25faa6fa6e3a200/fep/521b/fep-521b.md
func Test_FEP521b(t *testing.T) {
	if m := KeyRegex.FindStringSubmatch("https://invalid.invalid/.well-known/apgateway/did:key:u7QGwDY2Tjn93PVFWWq02piP1NE9_XRlg-c8-jhJiDqKBDw/actor#ed25519-key"); m == nil || m[1] != "u7QGwDY2Tjn93PVFWWq02piP1NE9_XRlg-c8-jhJiDqKBDw" {
		t.Fatalf("Failed to detect base64-encoded key")
	}

	if m := KeyRegex.FindStringSubmatch("https://invalid.invalid/.well-known/apgateway/did:key:z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2/actor#ed25519-key"); m == nil || m[1] != "z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2" {
		t.Fatalf("Failed to detect base58-encoded key")
	}
}

const testKey = "z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2"

func TestParseID_Forms(t *testing.T) {
	for _, tc := range []struct {
		id        string
		kind      IDKind
		origin    string
		host      string
		canonical string
		gateways  []string
	}{
		{"https://a.example/users/alice", URLID, "a.example", "a.example", "https://a.example/users/alice", nil},
		{"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor", GatewayID, "did:key:" + testKey, "a.example", "ap://did:key:" + testKey + "/actor", []string{"https://a.example"}},
		{"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor#ed25519-key", GatewayID, "did:key:" + testKey, "a.example", "ap://did:key:" + testKey + "/actor#ed25519-key", []string{"https://a.example"}},
		{"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor/inbox?123", GatewayID, "did:key:" + testKey, "a.example", "ap://did:key:" + testKey + "/actor/inbox", []string{"https://a.example"}},
		{"ap://did:key:" + testKey + "/actor", PortableID, "did:key:" + testKey, "", "ap://did:key:" + testKey + "/actor", nil},
		{"ap+ef61://did:key:" + testKey + "/actor", PortableID, "did:key:" + testKey, "", "ap://did:key:" + testKey + "/actor", nil},
		{"ap://did%3Akey%3A" + testKey + "/actor", PortableID, "did:key:" + testKey, "", "ap://did:key:" + testKey + "/actor", nil},
		{"ap+ef61://did%3akey%3a" + testKey + "/actor", PortableID, "did:key:" + testKey, "", "ap://did:key:" + testKey + "/actor", nil},
		{"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example", PortableID, "did:key:" + testKey, "a.example", "ap://did:key:" + testKey + "/actor", []string{"https://a.example"}},
		{"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example&@gateway=https%3A%2F%2Fb.example%2F#main-key", PortableID, "did:key:" + testKey, "a.example", "ap://did:key:" + testKey + "/actor#main-key", []string{"https://a.example", "https://b.example"}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			parsed, err := ParseID(tc.id)
			if err != nil {
				t.Fatalf("Failed to parse %s: %v", tc.id, err)
			}

			if parsed.Kind != tc.kind || parsed.Origin != tc.origin || parsed.Host != tc.host || parsed.Canonical != tc.canonical || !slices.Equal(parsed.Gateways, tc.gateways) {
				t.Fatalf("Unexpected result for %s: %+v", tc.id, parsed)
			}
		})
	}
}

func TestParseID_Invalid(t *testing.T) {
	for _, id := range []string{
		"ap://did:key:" + testKey,
		"ap://did:key%3A" + testKey + "/actor",
		"ap://did:key:" + testKey + "/actor?x=y",
		"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example&x=y",
		"ap://did:key:" + testKey + "/actor?@gateway=http%3A%2F%2Fa.example",
		"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example%2Fpath",
		"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example%3Fq",
		"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fu%40a.example",
		"ap://did:key:invalid/actor",
		"ap://did:web:a.example/actor",
		"https://a.example/.well-known/apgateway/did:key:" + testKey + "/x/.well-known/apgateway/did:key:" + testKey + "/actor",
	} {
		t.Run(id, func(t *testing.T) {
			if parsed, err := ParseID(id); err == nil {
				t.Fatalf("Parsed invalid ID %s: %+v", id, parsed)
			}
		})
	}
}

func TestValidateID(t *testing.T) {
	for _, id := range []string{
		"https://a.example/users/alice",
		"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor",
		"ap://did:key:" + testKey + "/actor",
		"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example",
	} {
		if _, err := ValidateID(id); err != nil {
			t.Fatalf("Failed to validate %s: %v", id, err)
		}
	}

	for _, id := range []string{
		"http://a.example/users/alice",
		"https://u@a.example/users/alice",
		"https://a.example/users/alice?x=y",
		"https://a.example/users/../alice",
		"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor?x=y",
		"ap://did:key:" + testKey + "/actor/../x",
		"did:key:" + testKey,
	} {
		if _, err := ValidateID(id); err == nil {
			t.Fatalf("Validated invalid ID %s", id)
		}
	}
}

func TestSameID(t *testing.T) {
	ids := []string{
		"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor",
		"https://b.example/.well-known/apgateway/did:key:" + testKey + "/actor",
		"ap://did:key:" + testKey + "/actor",
		"ap+ef61://did%3Akey%3A" + testKey + "/actor",
		"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example",
	}

	for _, a := range ids {
		for _, b := range ids {
			if !SameID(a, b) {
				t.Fatalf("%s and %s are not the same", a, b)
			}
		}
	}

	if SameID("ap://did:key:"+testKey+"/actor", "ap://did:key:"+testKey+"/actor#main-key") {
		t.Fatal("Different fragments are the same")
	}

	if SameID("https://a.example/users/alice", "https://b.example/users/alice") {
		t.Fatal("Different non-portable IDs are the same")
	}
}

func TestGateway(t *testing.T) {
	expected := "https://b.example/.well-known/apgateway/did:key:" + testKey + "/actor/inbox"

	for _, id := range []string{
		"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor/inbox",
		"ap://did:key:" + testKey + "/actor/inbox",
		"ap://did:key:" + testKey + "/actor/inbox?@gateway=https%3A%2F%2Fa.example",
	} {
		if gw := Gateway("https://b.example", id); gw != expected {
			t.Fatalf("Unexpected gateway URL for %s: %s", id, gw)
		}
	}

	if gw := Gateway("https://b.example", "https://a.example/users/alice"); gw != "https://a.example/users/alice" {
		t.Fatalf("Unexpected gateway URL for a non-portable ID: %s", gw)
	}
}

func TestWithGateways(t *testing.T) {
	id := WithGateways("ap://did:key:"+testKey+"/actor?@gateway=https%3A%2F%2Fc.example#main-key", []string{"https://a.example", "https://b.example"})
	if id != "ap://did:key:"+testKey+"/actor?@gateway=https%3A%2F%2Fa.example&@gateway=https%3A%2F%2Fb.example#main-key" {
		t.Fatalf("Unexpected ID: %s", id)
	}

	parsed, err := ParseID(id)
	if err != nil {
		t.Fatalf("Failed to parse %s: %v", id, err)
	}

	if !slices.Equal(parsed.Gateways, []string{"https://a.example", "https://b.example"}) {
		t.Fatalf("Unexpected gateways: %v", parsed.Gateways)
	}

	if id := WithGateways("https://a.example/.well-known/apgateway/did:key:"+testKey+"/actor", []string{"https://b.example"}); id != "https://a.example/.well-known/apgateway/did:key:"+testKey+"/actor" {
		t.Fatalf("Added hints to a compatible ID: %s", id)
	}
}
