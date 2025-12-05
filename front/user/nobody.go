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
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/httpsig"
)

// CreateNobody creates the special "nobdoy" user.
// This user is used to sign outgoing requests not initiated by a particular user.
func CreateNobody(ctx context.Context, domain string, db *sql.DB, cfg *cfg.Config) (*ap.Actor, [2]httpsig.Key, error) {
	var actor ap.Actor
	var rsaPrivKeyDer, ed25519PrivKey []byte
	if err := db.QueryRowContext(ctx, `select json(actor), rsaprivkey, ed25519privkey from persons where actor->>'$.preferredUsername' = 'nobody' and host = ?`, domain).Scan(&actor, &rsaPrivKeyDer, &ed25519PrivKey); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to create nobody user: %w", err)
	} else if err == nil {
		rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
		if err != nil {
			return nil, [2]httpsig.Key{}, err
		}

		return &actor, [2]httpsig.Key{
			{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
			{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
		}, err
	}

	return Create(ctx, domain, db, cfg, "nobody", ap.Application, nil)
}
