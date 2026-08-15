/*
Copyright 2023 - 2026 Dima Krasner

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
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

var errNoKeyInKeyID = errors.New("key origin does not contain a key")

func getKeyByID(actor *ap.Actor, keyID string) (crypto.PublicKey, error) {
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

		raw, err := data.DecodePublicKey(key.PublicKeyMultibase)
		if err != nil {
			return nil, fmt.Errorf("failed to parse %s: %w", key.ID, err)
		}

		return raw, nil
	}

	return nil, fmt.Errorf("key %s does not exist", keyID)
}

func (l *Listener) extractRequestSignature(r *http.Request, body []byte) (*httpsig.Signature, error) {
	sig, err := httpsig.Extract(r, body, l.Domain, time.Now(), l.Config.MaxRequestAge)
	if err != nil {
		return nil, fmt.Errorf("failed to extract signature: %w", err)
	}

	return sig, err
}

func (l *Listener) verifyRequestSignatureUsingKeyID(sig *httpsig.Signature) (string, error) {
	keyOrigin, err := ap.Origin(sig.KeyID)
	if err != nil {
		return "", fmt.Errorf("failed to get origin of %s: %w", sig.KeyID, err)
	}

	suffix, ok := strings.CutPrefix(keyOrigin, "did:key:")
	if !ok {
		return "", errNoKeyInKeyID
	}

	m := ap.KeyRegex.FindStringSubmatch(sig.KeyID)
	if m == nil {
		return "", errNoKeyInKeyID
	}

	if suffix != m[1] {
		return "", errNoKeyInKeyID
	}

	raw, err := data.DecodePublicKey(m[1])
	if err != nil {
		return "", fmt.Errorf("failed to parse %s: %w", sig.KeyID, err)
	}

	switch raw.(type) {
	case ed25519.PublicKey:
		if sig.Alg != "ed25519" {
			return "", errNoKeyInKeyID
		}

	case *mldsa44.PublicKey:
		if sig.Alg != "ml-dsa-44" {
			return "", errNoKeyInKeyID
		}

	default:
		return "", errNoKeyInKeyID
	}

	if err := sig.Verify(raw); err != nil {
		return "", fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
	}

	return m[1], nil
}

func (l *Listener) verifyRequestUsingKeyID(r *http.Request, body []byte) (*httpsig.Signature, string, error) {
	sig, err := l.extractRequestSignature(r, body)
	if err != nil {
		return nil, "", err
	}

	key, err := l.verifyRequestSignatureUsingKeyID(sig)
	return sig, key, err
}

func (l *Listener) verifyRequest(r *http.Request, body []byte, flags ap.ResolverFlag, keys [3]httpsig.Key) (*httpsig.Signature, *ap.Actor, error) {
	sig, err := l.extractRequestSignature(r, body)
	if err != nil {
		return nil, nil, err
	}

	if _, err := l.verifyRequestSignatureUsingKeyID(sig); err != nil && !errors.Is(err, errNoKeyInKeyID) {
		return nil, nil, err
	} else if err == nil {
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

	var publicKey crypto.PublicKey
	if actor.PublicKey.ID == sig.KeyID {
		publicKeyPem, _ := pem.Decode(danger.Bytes(actor.PublicKey.PublicKeyPem))
		if publicKeyPem == nil {
			return nil, nil, fmt.Errorf("failed to decode %s", sig.KeyID)
		}

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

func (l *Listener) verifyProof(ctx context.Context, activity *ap.Activity, raw []byte, flags ap.ResolverFlag, keys [3]httpsig.Key) (*ap.Actor, error) {
	if m := ap.KeyRegex.FindStringSubmatch(activity.Proof.VerificationMethod); m != nil {
		if m2 := ap.GatewayURLRegex.FindStringSubmatch(activity.Actor); m2 != nil {
			if m2[1] != m[1] {
				return nil, fmt.Errorf("key %s does not belong to %s", m[1], activity.Actor)
			}

			publicKey, err := data.DecodePublicKey(m[1])
			if err != nil {
				return nil, fmt.Errorf("failed to decode key %s to verify proof: %w", activity.Proof.VerificationMethod, err)
			}

			if err := proof.Verify(publicKey, activity.Proof, activity.Context, raw); err != nil {
				return nil, fmt.Errorf("failed to verify proof using %s: %w", activity.Proof.VerificationMethod, err)
			}

			return l.Resolver.ResolveID(ctx, keys, activity.Actor, flags)
		}
	}

	actor, err := l.Resolver.ResolveID(ctx, keys, activity.Proof.VerificationMethod, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s to verify proof: %w", activity.Proof.VerificationMethod, err)
	}

	if actor.ID != activity.Actor {
		return nil, fmt.Errorf("key %s belongs to %s, not %s", activity.Proof.VerificationMethod, actor.ID, activity.Actor)
	}

	publicKey, err := getKeyByID(actor, activity.Proof.VerificationMethod)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s to verify proof: %w", activity.Proof.VerificationMethod, err)
	}

	if err := proof.Verify(publicKey, activity.Proof, activity.Context, raw); err != nil {
		return nil, fmt.Errorf("failed to verify proof using %s: %w", activity.Proof.VerificationMethod, err)
	}

	return actor, nil
}
