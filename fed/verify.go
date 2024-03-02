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
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/httpsig"
	"log/slog"
	"net/http"
)

func verify(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, r *http.Request, body []byte, db *sql.DB, resolver *Resolver, key httpsig.Key, flags ap.ResolverFlag) (*ap.Actor, error) {
	sig, err := httpsig.Extract(r, body, domain, cfg.MaxRequestAge)
	if err != nil {
		return nil, fmt.Errorf("failed to verify message: %w", err)
	}

	actor, err := resolver.ResolveID(r.Context(), log, db, key, sig.KeyID, flags)
	if err != nil {
		return nil, fmt.Errorf("failed to get key %s to verify message: %w", sig.KeyID, err)
	}

	publicKeyPem, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))

	publicKey, err := x509.ParsePKIXPublicKey(publicKeyPem.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
	}

	if err := sig.Verify(publicKey); err != nil {
		return nil, fmt.Errorf("failed to verify message using %s: %w", sig.KeyID, err)
	}

	return actor, nil
}
