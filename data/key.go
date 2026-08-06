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
	"crypto/mldsa"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
)

// EncodeEd25519PrivateKey encodes an Ed25519 private key.
func EncodeEd25519PrivateKey(key ed25519.PrivateKey) string {
	return "z" + base58.Encode(append([]byte{0x80, 0x26}, key.Seed()...))
}

// DecodeEd25519PrivateKey decodes an Ed25519 private key encoded by [EncodeEd25519PrivateKey].
func DecodeEd25519PrivateKey(key string) (ed25519.PrivateKey, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}

	if key[0] != 'z' {
		return nil, fmt.Errorf("invalid key prefix: %c", key[0])
	}

	rawKey := base58.Decode(key[1:])

	if len(rawKey) != ed25519.SeedSize+2 {
		return nil, fmt.Errorf("invalid key length: %c", len(rawKey))
	}

	if rawKey[0] != 0x80 || rawKey[1] != 0x26 {
		return nil, fmt.Errorf("invalid key prefix: %02x%02x", rawKey[0], rawKey[1])
	}

	return ed25519.NewKeyFromSeed(rawKey[2:]), nil
}

// EncodeEd25519PublicKey encodes an Ed25519 public key.
func EncodeEd25519PublicKey(key ed25519.PublicKey) string {
	return "z" + base58.Encode(append([]byte{0xed, 0x01}, key...))
}

// DecodeEd25519PublicKey decodes an Ed25519 public key encoded by [EncodeEd25519PublicKey].
func DecodeEd25519PublicKey(key string) (ed25519.PublicKey, error) {
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
		return nil, fmt.Errorf("invalid prefix: %d", key[0])
	}

	if len(rawKey) != ed25519.PublicKeySize+2 {
		return nil, fmt.Errorf("invalid key length: %d", len(rawKey))
	}

	if rawKey[0] != 0xed || rawKey[1] != 0x01 {
		return nil, fmt.Errorf("invalid prefix: %x%x", rawKey[0], rawKey[1])
	}

	return ed25519.PublicKey(rawKey[2:]), nil
}

// EncodeMLDSA44PrivateKey encodes a ML-DSA-44 private key.
func EncodeMLDSA44PrivateKey(key *mldsa.PrivateKey) string {
	return "u" + base64.RawURLEncoding.EncodeToString(append([]byte{0x13, 0x1a}, key.Bytes()...))
}

// DecodeMLDSA44PrivateKey decodes an ML-DSA-44 private key encoded by [EncodeMLDSA44PrivateKey].
func DecodeMLDSA44PrivateKey(key string) (*mldsa.PrivateKey, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}

	if key[0] != 'u' {
		return nil, fmt.Errorf("invalid key prefix: %c", key[0])
	}

	seed, err := base64.RawURLEncoding.DecodeString(key[1:])
	if err != nil {
		return nil, fmt.Errorf("failed to decode key: %w", err)
	}

	if len(seed) != 2+32 {
		return nil, fmt.Errorf("invalid key length: %d", len(seed))
	}

	if seed[0] != 0x13 || seed[1] != 0x1a {
		return nil, fmt.Errorf("invalid key prefix: %02x%02x", seed[0], seed[1])
	}

	return mldsa.NewPrivateKey(mldsa.MLDSA44(), seed[2:])
}

// EncodeMLDSA44Publickey encodes a ML-DSA-44 public key.
func EncodeMLDSA44Publickey(key *mldsa.PublicKey) string {
	return "u" + base64.RawURLEncoding.EncodeToString(append([]byte{0x90, 0x24}, key.Bytes()...))
}

// DecodeMLDSA44PublicKey decodes an ML-DSA-44 public key encoded by [EncodeMLDSA44Publickey].
func DecodeMLDSA44PublicKey(key string) (*mldsa.PublicKey, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}

	var raw []byte
	switch key[0] {
	case 'u':
		var err error
		raw, err = base64.RawURLEncoding.DecodeString(key[1:])
		if err != nil {
			return nil, fmt.Errorf("failed to decode key: %w", err)
		}

	case 'z':
		raw = base58.Decode(key[1:])

	default:
		return nil, fmt.Errorf("invalid key prefix: %c", key[0])
	}

	if len(raw) != 2+mldsa.MLDSA44PublicKeySize {
		return nil, fmt.Errorf("invalid key length: %d", len(raw))
	}

	if raw[0] != 0x90 || raw[1] != 0x24 {
		return nil, fmt.Errorf("invalid key prefix: %02x%02x", raw[0], raw[1])
	}

	return mldsa.NewPublicKey(mldsa.MLDSA44(), raw[2:])
}

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
		return nil, fmt.Errorf("invalid prefix: %d", key[0])
	}

	switch len(rawKey) {
	case 2 + ed25519.PublicKeySize:
		if rawKey[0] != 0xed || rawKey[1] != 0x01 {
			return nil, fmt.Errorf("invalid prefix: %x%x", rawKey[0], rawKey[1])
		}

		return ed25519.PublicKey(rawKey[2:]), nil

	case 2 + mldsa.MLDSA44PublicKeySize:
		if rawKey[0] != 0x90 || rawKey[1] != 0x24 {
			return nil, fmt.Errorf("invalid prefix: %x%x", rawKey[0], rawKey[1])
		}

		return mldsa.NewPublicKey(mldsa.MLDSA44(), rawKey[2:])

	default:
		return nil, fmt.Errorf("invalid key length: %d", len(rawKey))
	}
}
