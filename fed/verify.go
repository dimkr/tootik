/*
Copyright 2023, 2024 Dima Krasner

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
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/go-fed/httpsig"
	"log/slog"
	"net/http"
)

func verify(ctx context.Context, domain string, log *slog.Logger, r *http.Request, db *sql.DB, resolver *Resolver, from *ap.Actor, offline bool) (*ap.Actor, error) {
	sig := r.Header.Get("Signature")
	if sig == "" {
		return nil, errors.New("failed to verify message: no signature")
	}

	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		return nil, fmt.Errorf("failed to verify message: %w", err)
	}

	keyID := verifier.KeyId()

	actor, err := resolver.Resolve(r.Context(), log, db, from, keyID, offline)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s to verify message: %w", keyID, err)
	}

	publicKeyPem, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))

	publicKey, err := x509.ParsePKIXPublicKey(publicKeyPem.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to verify message using %s: %w", keyID, err)
	}

	if r.Header.Get("Host") == "" {
		r.Header.Add("Host", domain)
	}

	if err := verifier.Verify(publicKey, httpsig.RSA_SHA256); err != nil {
		return nil, fmt.Errorf("failed to verify message using %s: %w", keyID, err)
	}

	return actor, nil
}
