/*
Copyright 2023 Dima Krasner

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
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/go-fed/httpsig"
	"net/http"
)

func verify(ctx context.Context, actorID string, r *http.Request, db *sql.DB) error {
	actor, err := Resolve(r.Context(), db, nil, actorID)
	if err != nil {
		return fmt.Errorf("Failed get key for %s: %w", actorID, err)
	}

	keyID := string(actor.PublicKey.ID.GetLink())

	verifier, err := httpsig.NewVerifier(r)
	if err != nil {
		return fmt.Errorf("Failed to verify message from %s using %s: %w", actorID, keyID, err)
	}

	publicKeyPem, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))

	publicKey, err := x509.ParsePKIXPublicKey(publicKeyPem.Bytes)
	if err != nil {
		return fmt.Errorf("Failed to verify message from %s using %s: %w", actorID, keyID, err)
	}

	if r.Header.Get("Host") == "" {
		r.Header.Add("Host", cfg.Domain)
	}

	if err := verifier.Verify(publicKey, httpsig.RSA_SHA256); err != nil {
		return fmt.Errorf("Failed to verify message from %s using %s: %w", actorID, keyID, err)
	}

	return nil
}
