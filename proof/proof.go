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

// Package proof creates and verifies integrity proofs.
//
// See https://codeberg.org/fediverse/fep/src/branch/main/fep/8b32/fep-8b32.md for more details.
package proof

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/btcsuite/btcutil/base58"
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

// Create creates an eddsa-jcs-2022 integrity proof for a JSON object.
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

	default:
		return ap.Proof{}, fmt.Errorf("cannot create proof for %T", v)
	}
}

func create(key httpsig.Key, now time.Time, doc, context any) (ap.Proof, error) {
	edKey, ok := key.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return ap.Proof{}, fmt.Errorf("wrong key type: %T", key.PrivateKey)
	}

	created := now.UTC().Format(time.RFC3339)

	keyID := key.ID
	if m := ap.GatewayURLRegex.FindStringSubmatch(keyID); m != nil {
		keyID = fmt.Sprintf("did:key:%s#%s", m[1], m[1])
	}

	proof := ap.Proof{
		Context:            context,
		Type:               "DataIntegrityProof",
		CryptoSuite:        "eddsa-jcs-2022",
		Created:            created,
		Purpose:            "assertionMethod",
		VerificationMethod: keyID,
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

	proof.Value = "z" + base58.Encode(ed25519.Sign(edKey, append(cfgHash[:], docHash[:]...)))
	return proof, nil
}

// Add adds an eddsa-jcs-2022 integrity proof to a JSON object.
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
func Verify(key any, proof ap.Proof, raw []byte) error {
	edKey, ok := key.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("wrong key type: %T", key)
	}

	if proof.Type != "DataIntegrityProof" {
		return errors.New("invalid type: " + proof.Type)
	}

	if proof.CryptoSuite != "eddsa-jcs-2022" {
		return errors.New("invalid cryptosuite: " + proof.CryptoSuite)
	}

	if proof.Purpose != "assertionMethod" {
		return errors.New("invalid purpose: " + proof.Purpose)
	}

	if len(proof.Value) <= 1 || proof.Value[0] != 'z' {
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

	cfg, err := normalizeJSON(options)
	if err != nil {
		return err
	}

	cfgHash := sha256.Sum256(cfg)

	if !ed25519.Verify(edKey, append(cfgHash[:], docHash[:]...), base58.Decode(proof.Value[1:])) {
		return errors.New("proof verification failed")
	}

	return nil
}
