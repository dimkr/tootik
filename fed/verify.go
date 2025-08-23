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
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func getKeyByID(actor *ap.Actor, keyID string) (ed25519.PublicKey, error) {
	for _, key := range actor.AssertionMethod {
		if key.ID != keyID {
			continue
		}

		if key.Type != "Multikey" {
			continue
		}

		if key.Controller != actor.ID {
			continue
		}

		raw, err := data.DecodeEd25519PublicKey(key.PublicKeyMultibase)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", key.ID, err)
		}

		return raw, nil
	}

	return nil, fmt.Errorf("key %s does not exist", keyID)
}

func (l *Listener) verifyRequest(r *http.Request, body []byte, flags ap.ResolverFlag, keys [2]httpsig.Key) (*httpsig.Signature, *ap.Actor, error) {
	sig, err := httpsig.Extract(r, body, l.Domain, time.Now(), l.Config.MaxRequestAge)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to verify message: %w", err)
	}

	if m := ap.KeyRegex.FindStringSubmatch(sig.KeyID); m != nil {
		raw, err := data.DecodeEd25519PublicKey(m[1])
		if err != nil {
			return nil, nil, fmt.Errorf("failed to parse %s: %w", sig.KeyID, err)
		}

		if err := sig.Verify(raw); err != nil {
			return nil, nil, fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
		}

		actor, err := l.Resolver.ResolveID(r.Context(), keys, sig.KeyID, flags)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to fetch %s: %w", sig.KeyID, err)
		}

		return sig, actor, nil
	}

	actor, err := l.Resolver.ResolveID(r.Context(), keys, sig.KeyID, flags)
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
		publicKey, err = getKeyByID(actor, sig.KeyID)
		if err != nil {
			return nil, nil, err
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

func (l *Listener) verifyProof(ctx context.Context, p ap.Proof, activity *ap.Activity, raw []byte, flags ap.ResolverFlag, keys [2]httpsig.Key) (*ap.Actor, error) {
	if m := ap.KeyRegex.FindStringSubmatch(p.VerificationMethod); m != nil {
		publicKey, err := data.DecodeEd25519PublicKey(m[1])
		if err != nil {
			return nil, fmt.Errorf("failed to get key %s to verify proof: %w", p.VerificationMethod, err)
		}

		if err := proof.Verify(publicKey, activity.Proof, activity.Context, raw); err != nil {
			return nil, fmt.Errorf("failed to verify proof using %s: %w", p.VerificationMethod, err)
		}

		return l.Resolver.ResolveID(ctx, keys, activity.Actor, flags)
	}

	actor, err := l.Resolver.ResolveID(ctx, keys, p.VerificationMethod, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s to verify proof: %w", p.VerificationMethod, err)
	}

	publicKey, err := getKeyByID(actor, p.VerificationMethod)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s to verify proof: %w", p.VerificationMethod, err)
	}

	if err := proof.Verify(publicKey, p, activity.Context, raw); err != nil {
		return nil, fmt.Errorf("failed to verify proof using %s: %w", p.VerificationMethod, err)
	}

	return actor, nil
}
