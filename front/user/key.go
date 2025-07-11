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

package user

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
)

// KeyGenerator generates a keypair for a newly created user.
type KeyGenerator func() (any, []byte, []byte, error)

// GenerateED25519Key generates an Ed25519 keypair for a newly created user.
func GenerateED25519Key() (any, []byte, []byte, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	privPkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to marshal private key: %w", err)
	}

	var privPem bytes.Buffer
	if err := pem.Encode(
		&privPem,
		&pem.Block{
			Type:  "BEGIN PRIVATE KEY",
			Bytes: privPkcs8,
		},
	); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	pubPkix, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	var pubPem bytes.Buffer
	if err := pem.Encode(
		&pubPem,
		&pem.Block{
			Type:  "BEGIN  PUBLIC KEY",
			Bytes: pubPkix,
		},
	); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	return priv, privPem.Bytes(), pubPem.Bytes(), nil
}

// GenerateRSAKey generates an RSA keypair for a newly created user.
func GenerateRSAKey() (any, []byte, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	var privPem bytes.Buffer
	if err := pem.Encode(
		&privPem,
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		},
	); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	var pubPem bytes.Buffer
	if err := pem.Encode(
		&pubPem,
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(&priv.PublicKey),
		},
	); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate public key: %w", err)
	}

	return priv, privPem.Bytes(), pubPem.Bytes(), nil
}
