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

package data

import (
	"crypto"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
)

type PrivateKey interface {
	crypto.PrivateKey

	Public() crypto.PublicKey
}

// EncodeEd25519PrivateKey encodes an Ed25519 private key.
func EncodeEd25519PrivateKey(key ed25519.PrivateKey) string {
	return "z" + base58.Encode(append([]byte{0x80, 0x26}, key.Seed()...))
}

// EncodeEd25519PublicKey encodes an Ed25519 public key.
func EncodeEd25519PublicKey(key ed25519.PublicKey) string {
	return "z" + base58.Encode(append([]byte{0xed, 0x01}, key...))
}

// EncodeMLDSA44PrivateKey encodes a ML-DSA-44 private key.
func EncodeMLDSA44PrivateKey(key *mldsa44.PrivateKey) string {
	return "u" + base64.RawURLEncoding.EncodeToString(append([]byte{0x13, 0x1a}, key.Seed()...))
}

// EncodeMLDSA44Publickey encodes a ML-DSA-44 public key.
func EncodeMLDSA44Publickey(key *mldsa44.PublicKey) string {
	return "u" + base64.RawURLEncoding.EncodeToString(append([]byte{0x90, 0x24}, key.Bytes()...))
}

// DecodePrivateKey decodes a public key encoded by [EncodeEd25519PrivateKey] or [EncodeMLDSA44PrivateKey].
func DecodePrivateKey(key string) (PrivateKey, error) {
	if len(key) == 0 {
		return nil, errors.New("key is empty")
	}

	var rawKey []byte
	switch key[0] {
	case 'z':
		rawKey = base58.Decode(key[1:])

	case 'u':
		var err error
		rawKey, err = base64.RawURLEncoding.DecodeString(key[1:])
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %w", err)
		}

	default:
		return nil, fmt.Errorf("invalid prefix: %c", key[0])
	}

	if len(rawKey) == 2+ed25519.SeedSize && rawKey[0] == 0x80 && rawKey[1] == 0x26 {
		return ed25519.NewKeyFromSeed(rawKey[2:]), nil
	} else if len(rawKey) == 2+mldsa44.SeedSize && rawKey[0] == 0x13 && rawKey[1] == 0x1a {
		_, priv := mldsa44.NewKeyFromSeed((*[mldsa44.SeedSize]byte)(rawKey[2:]))
		return priv, nil
	} else if len(rawKey) >= 2 {
		return nil, fmt.Errorf("invalid key prefix: %02x%02x", rawKey[0], rawKey[1])
	} else {
		return nil, fmt.Errorf("invalid key length: %d", len(rawKey))
	}
}

// DecodePublicKey decodes a public key encoded by [EncodeEd25519PublicKey] or [EncodeMLDSA44PublicKey].
func DecodePublicKey(key string) (crypto.PublicKey, error) {
	if len(key) == 0 {
		return nil, errors.New("key is empty")
	}

	var rawKey []byte
	switch key[0] {
	case 'z':
		rawKey = base58.Decode(key[1:])

	case 'u':
		var err error
		rawKey, err = base64.RawURLEncoding.DecodeString(key[1:])
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %w", err)
		}

	default:
		return nil, fmt.Errorf("invalid prefix: %c", key[0])
	}

	switch len(rawKey) {
	case 2 + ed25519.PublicKeySize:
		if rawKey[0] != 0xed || rawKey[1] != 0x01 {
			return nil, fmt.Errorf("invalid prefix: %02x%02x", rawKey[0], rawKey[1])
		}

		return ed25519.PublicKey(rawKey[2:]), nil

	case 2 + mldsa44.PublicKeySize:
		if rawKey[0] != 0x90 || rawKey[1] != 0x24 {
			return nil, fmt.Errorf("invalid prefix: %02x%02x", rawKey[0], rawKey[1])
		}

		pub := &mldsa44.PublicKey{}
		return pub, pub.UnmarshalBinary(rawKey[2:])

	default:
		return nil, fmt.Errorf("invalid key length: %d", len(rawKey))
	}
}
