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

var proofContext = []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v1"}

func normalizeJSON(v any) ([]byte, error) {
	j, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	return jcs.Transform(j)
}

func create(key httpsig.Key, now time.Time, doc, context any) (ap.Proof, error) {
	edKey, ok := key.PrivateKey.(ed25519.PrivateKey)
	if !ok {
		return ap.Proof{}, fmt.Errorf("wrong key type: %T", key.PrivateKey)
	}

	created := now.UTC().Format(time.RFC3339)

	keyID := key.ID
	if m := ap.CompatibleURLRegex.FindStringSubmatch(keyID); m != nil {
		keyID = "did:key:" + m[1]
	}

	cfg, err := normalizeJSON(map[string]any{
		"@context":           context,
		"type":               "DataIntegrityProof",
		"cryptosuite":        "eddsa-jcs-2022",
		"created":            created,
		"proofPurpose":       "assertionMethod",
		"verificationMethod": keyID,
	})
	if err != nil {
		return ap.Proof{}, err
	}

	data, err := normalizeJSON(doc)
	if err != nil {
		return ap.Proof{}, err
	}

	cfgHash := sha256.Sum256(cfg)
	docHash := sha256.Sum256(data)

	return ap.Proof{
		Type:               "DataIntegrityProof",
		CryptoSuite:        "eddsa-jcs-2022",
		VerificationMethod: keyID,
		Purpose:            "assertionMethod",
		Value:              "z" + base58.Encode(ed25519.Sign(edKey, append(cfgHash[:], docHash[:]...))),
		Created:            created,
	}, nil
}

// Add adds an eddsa-jcs-2022 integrity proof to a JSON object.
func Add(key httpsig.Key, now time.Time, raw []byte) ([]byte, error) {
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}

	m["@context"] = proofContext

	proof, err := create(key, now, m, proofContext)
	if err != nil {
		return nil, err
	}

	m["proof"] = proof
	return json.Marshal(m)
}

func tryVerify(key ed25519.PublicKey, docHash [32]byte, proof ap.Proof, context any) (bool, error) {
	m := map[string]any{
		"type":               proof.Type,
		"cryptosuite":        proof.CryptoSuite,
		"created":            proof.Created,
		"proofPurpose":       proof.Purpose,
		"verificationMethod": proof.VerificationMethod,
	}

	if context != nil {
		m["@context"] = context
	}

	cfg, err := normalizeJSON(m)
	if err != nil {
		return false, err
	}

	cfgHash := sha256.Sum256(cfg)

	if ed25519.Verify(key, append(cfgHash[:], docHash[:]...), base58.Decode(proof.Value[1:])) {
		return true, nil
	}

	return false, nil
}

// Verify verifies an integrity proof.
func Verify(key any, proof ap.Proof, context any, raw []byte) error {
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

	if ok, err := tryVerify(edKey, docHash, proof, context); err != nil {
		return err
	} else if ok {
		return nil
	}

	/*
		try again without @context, because Hubzilla ignores it
		https://framagit.org/hubzilla/core/-/blob/aaa863cda7c29daa4fe0322749f55f50e2123d1d/Zotlabs/Lib/JcsEddsa2022.php#L34
	*/
	if context != nil {
		if ok, err := tryVerify(edKey, docHash, proof, nil); err != nil {
			return err
		} else if ok {
			return nil
		}
	}

	return errors.New("proof verification failed")
}
