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

package proof

import (
	"crypto/ed25519"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

// SigningKey the key that should be used to create proofs on behalf of actor.
func SigningKey(id string, keys [3]httpsig.Key) httpsig.Key {
	return keys[1]
}

// SigningSeed the key that should be used to create proofs on behalf of actor.
func SigningSeed(actor *ap.Actor, ed25519Seed, mldsa44Seed []byte) httpsig.Key {
	return httpsig.Key{
		ID:         actor.AssertionMethod[0].ID,
		PrivateKey: ed25519.NewKeyFromSeed(ed25519Seed),
	}
}
