/*
Copyright 2025 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless ruired by applicable law or agreed to in writing, software
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

// Create creates an eddsa-jcs-2022 integrity proof for a given [ap.Activity].
func Create(key httpsig.Key, now time.Time, activity *ap.Activity) (ap.Proof, error) {
	edKey, ok := key.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return ap.Proof{}, fmt.Errorf("wrong key type: %T", key.PrivateKey)
	}

	created := now.UTC().Format("2006-01-02T15:04:05")

	cfg, err := normalizeJSON(map[string]any{
		"@context":           activity.Context,
		"type":               "DataIntegrityProof",
		"cryptosuite":        "eddsa-jcs-2022",
		"created":            created,
		"proofPurpose":       "assertionMethod",
		"verificationMethod": key.ID,
	})
	if err != nil {
		return ap.Proof{}, err
	}

	data, err := normalizeJSON(activity)
	if err != nil {
		return ap.Proof{}, err
	}

	cfgHash := sha256.Sum256(cfg)
	docHash := sha256.Sum256(data)

	return ap.Proof{
		Context:            activity.Context,
		Type:               "DataIntegrityProof",
		CryptoSuite:        "eddsa-jcs-2022",
		VerificationMethod: key.ID,
		Purpose:            "assertionMethod",
		Value:              "z" + base58.Encode(ed25519.Sign(edKey, append(cfgHash[:], docHash[:]...))),
		Created:            created,
	}, nil
}

// Verify verifies an integrity proof.
func Verify(key any, activity *ap.Activity, raw []byte) error {
	edKey, ok := key.(ed25519.PublicKey)
	if !ok {
		return fmt.Errorf("wrong key type: %T", key)
	}

	if activity.Proof.Type != "DataIntegrityProof" {
		return errors.New("invalid type: " + activity.Proof.Type)
	}

	if activity.Proof.CryptoSuite != "eddsa-jcs-2022" {
		return errors.New("invalid cryptosuite: " + activity.Proof.CryptoSuite)
	}

	if activity.Proof.Purpose != "assertionMethod" {
		return errors.New("invalid purpose: " + activity.Proof.Purpose)
	}

	if len(activity.Proof.Value) <= 1 || activity.Proof.Value[0] != 'z' {
		return errors.New("invalid value: " + activity.Proof.Value)
	}

	cfg, err := normalizeJSON(map[string]any{
		"@context":           activity.Context,
		"type":               "DataIntegrityProof",
		"cryptosuite":        "eddsa-jcs-2022",
		"created":            activity.Proof.Created,
		"proofPurpose":       "assertionMethod",
		"verificationMethod": activity.Proof.VerificationMethod,
	})
	if err != nil {
		return err
	}

	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		return err
	}
	delete(m, "proof")

	j, err := json.Marshal(m)
	if err != nil {
		return err
	}

	data, err := jcs.Transform(j)
	if err != nil {
		return err
	}

	cfgHash := sha256.Sum256(cfg)
	docHash := sha256.Sum256(data)

	if !ed25519.Verify(edKey, append(cfgHash[:], docHash[:]...), base58.Decode(activity.Proof.Value[1:])) {
		return errors.New("proof verification failed")
	}

	return nil
}
