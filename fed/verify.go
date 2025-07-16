/*
Copyright 2023 - 2025 Dima Krasner

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

package fed

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
)

func (l *Listener) verify(r *http.Request, body []byte, flags ap.ResolverFlag) (*httpsig.Signature, *ap.Actor, error) {
	sig, err := httpsig.Extract(r, body, l.Domain, time.Now(), l.Config.MaxRequestAge)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify message: %w", err)
	}

	actor, err := l.Resolver.ResolveID(r.Context(), l.ActorKeys, sig.KeyID, flags)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get key %s to verify message: %w", sig.KeyID, err)
	}

	var publicKey any
	if actor.PublicKey.ID == sig.KeyID {
		publicKeyPem, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))

		var err error
		publicKey, err = x509.ParsePKIXPublicKey(publicKeyPem.Bytes)
		if err != nil {
			publicKey, err = x509.ParsePKCS1PublicKey(publicKeyPem.Bytes)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
			}
		}
	} else {
		for _, key := range actor.AssertionMethod {
			if key.ID != sig.KeyID {
				continue
			}

			if key.Type != "Multikey" {
				continue
			}

			if key.Controller != actor.ID {
				continue
			}

			if len(key.PublicKeyMultibase) == 0 {
				return nil, nil, fmt.Errorf("key %s is empty", key.ID)
			}

			if key.PublicKeyMultibase[0] != 'z' {
				return nil, nil, fmt.Errorf("invalid prefix for %s: %c", key.ID, key.PublicKeyMultibase[0])
			}

			rawKey := base58.Decode(key.PublicKeyMultibase[1:])

			if len(rawKey) != ed25519.PublicKeySize+2 {
				return nil, nil, fmt.Errorf("invalid key length for %s: %d", key.ID, len(rawKey))
			}

			if rawKey[0] != 0xed || rawKey[1] != 0x01 {
				return nil, nil, fmt.Errorf("invalid prefix for %s: %x%x", key.ID, rawKey[0], rawKey[1])
			}

			publicKey = ed25519.PublicKey(rawKey[2:])
		}
	}

	if publicKey == nil {
		return nil, nil, errors.New("cannot verify message using non-existing key " + sig.KeyID)
	}

	if err := sig.Verify(publicKey); err != nil {
		return nil, nil, fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
	}

	return sig, actor, nil
}
