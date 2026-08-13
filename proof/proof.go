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

// Package proof creates and verifies integrity proofs.
//
// See https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md for more details.
package proof

import (
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcutil/base58"
	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/gowebpki/jcs"
)

func normalizeJSON(v any) ([]byte, error) {
	j, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return jcs.Transform(j)
}

// Create creates an integrity proof for a JSON object.
func Create(key httpsig.Key, doc any) (ap.Proof, error) {
	switch v := doc.(type) {
	case *ap.Activity:
		clone := *v
		clone.Proof = ap.Proof{}
		return create(key, time.Now(), &clone, clone.Context)

	case *ap.Object:
		clone := *v
		clone.Proof = ap.Proof{}
		return create(key, time.Now(), &clone, clone.Context)

	case *ap.Actor:
		clone := *v
		clone.Proof = ap.Proof{}
		return create(key, time.Now(), &clone, clone.Context)

	case *ap.Collection:
		clone := *v
		clone.Proof = ap.Proof{}
		return create(key, time.Now(), &clone, clone.Context)

	case *ap.CollectionPage:
		clone := *v
		clone.Proof = ap.Proof{}
		return create(key, time.Now(), &clone, clone.Context)

	default:
		return ap.Proof{}, fmt.Errorf("cannot create proof for %T", v)
	}
}

func create(key httpsig.Key, now time.Time, doc, context any) (ap.Proof, error) {
	created := now.UTC().Format(time.RFC3339)

	keyID := key.ID
	if m := ap.GatewayURLRegex.FindStringSubmatch(keyID); m != nil {
		keyID = fmt.Sprintf("did:key:%s#%s", m[1], m[1])
	}

	proof := ap.Proof{
		Context:            context,
		Type:               "DataIntegrityProof",
		Created:            created,
		Purpose:            "assertionMethod",
		VerificationMethod: keyID,
	}

	switch key.PrivateKey.(type) {
	case ed25519.PrivateKey:
		proof.CryptoSuite = "eddsa-jcs-2022"

	case *mldsa44.PrivateKey:
		proof.CryptoSuite = "mldsa44-jcs-2024"

	default:
		return ap.Proof{}, fmt.Errorf("wrong key type: %T", key.PrivateKey)
	}

	cfg, err := normalizeJSON(proof)
	if err != nil {
		return ap.Proof{}, err
	}

	data, err := normalizeJSON(doc)
	if err != nil {
		return ap.Proof{}, err
	}

	cfgHash := sha256.Sum256(cfg)
	docHash := sha256.Sum256(data)

	switch v := key.PrivateKey.(type) {
	case ed25519.PrivateKey:
		proof.Value = "z" + base58.Encode(ed25519.Sign(v, append(cfgHash[:], docHash[:]...)))

	case *mldsa44.PrivateKey:
		sig := make([]byte, mldsa44.SignatureSize)
		if err := mldsa44.SignTo(v, append(cfgHash[:], docHash[:]...), nil, true, sig); err != nil {
			return ap.Proof{}, err
		}

		proof.Value = "u" + base64.RawURLEncoding.EncodeToString(sig)
	}

	return proof, nil
}

// Add adds an integrity proof to a JSON object.
func Add(key httpsig.Key, now time.Time, raw []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	proof, err := create(key, now, m, m["@context"])
	if err != nil {
		return nil, err
	}

	m["proof"] = proof
	return json.Marshal(m)
}

// Verify verifies an integrity proof.
func Verify(key crypto.PublicKey, proof ap.Proof, context any, raw []byte) error {
	if proof.Type != "DataIntegrityProof" {
		return errors.New("invalid type: " + proof.Type)
	}

	if proof.Purpose != "assertionMethod" {
		return errors.New("invalid purpose: " + proof.Purpose)
	}

	if len(proof.Value) <= 1 {
		return errors.New("invalid value: " + proof.Value)
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	delete(m, "proof")
	delete(m, "signature")

	j, err := json.Marshal(m)
	if err != nil {
		return err
	}

	data, err := jcs.Transform(j)
	if err != nil {
		return err
	}

	docHash := sha256.Sum256(data)

	options := proof
	options.Value = ""

	switch proof.CryptoSuite {
	case "eddsa-jcs-2022":
		if options.Context == nil {
			options.Context = context
		}

	case "mldsa44-jcs-2024":
		options.Context = context

	default:
		return fmt.Errorf("invalid cryptosuite: %s/%T", proof.CryptoSuite, key)
	}

	cfg, err := normalizeJSON(options)
	if err != nil {
		return err
	}

	cfgHash := sha256.Sum256(cfg)

	switch proof.CryptoSuite {
	case "eddsa-jcs-2022":
		if proof.Value[0] != 'z' {
			return errors.New("invalid value: " + proof.Value)
		}

		edKey, ok := key.(ed25519.PublicKey)
		if !ok {
			return fmt.Errorf("wrong key type: %T", key)
		}

		if !ed25519.Verify(edKey, append(cfgHash[:], docHash[:]...), base58.Decode(proof.Value[1:])) {
			return errors.New("proof verification failed")
		}

	case "mldsa44-jcs-2024":
		if proof.Value[0] != 'u' {
			return errors.New("invalid value: " + proof.Value)
		}

		mlKey, ok := key.(*mldsa44.PublicKey)
		if !ok {
			return fmt.Errorf("wrong key type: %T", key)
		}

		sig, err := base64.RawURLEncoding.DecodeString(proof.Value[1:])
		if err != nil {
			return fmt.Errorf("failed to decode proof: %w", err)
		}

		if !mldsa44.Verify(mlKey, append(cfgHash[:], docHash[:]...), nil, sig) {
			return errors.New("proof verification failed")
		}
	}

	return nil
}
