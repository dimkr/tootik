/*
Copyright 2026 Dima Krasner

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

// Package proof creates and verifies integrity proofs.
//
// See https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md for more details.
package proof

import (
	"crypto/ed25519"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
)

func SigningKey(id string, keys [3]httpsig.Key) httpsig.Key {
	m := ap.GatewayURLRegex.FindStringSubmatch(id)
	if m == nil {
		return keys[1]
	}

	pub, err := data.DecodePublicKey(m[1])
	if err != nil {
		return keys[1]
	}

	if _, ok := pub.(*mldsa44.PublicKey); ok {
		return keys[2]
	}

	return keys[1]
}

func SigningSeed(actor *ap.Actor, ed25519Seed, mldsa44Seed []byte) httpsig.Key {
	if m := ap.GatewayURLRegex.FindStringSubmatch(actor.ID); m != nil {
		if pub, err := data.DecodePublicKey(m[1]); err == nil {
			if _, ok := pub.(*mldsa44.PublicKey); ok {
				_, priv := mldsa44.NewKeyFromSeed((*[mldsa44.SeedSize]byte)(mldsa44Seed))
				return httpsig.Key{
					ID:         actor.AssertionMethod[1].ID,
					PrivateKey: priv,
				}
			}
		}
	}

	return httpsig.Key{
		ID:         actor.AssertionMethod[0].ID,
		PrivateKey: ed25519.NewKeyFromSeed(ed25519Seed),
	}
}
