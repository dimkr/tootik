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
	"database/sql"
	"errors"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
)

// CreateNobody creates the special "nobdoy" user.
// This user is used to sign outgoing requests not initiated by a particular user.
func CreateNobody(ctx context.Context, domain string, db *sql.DB) (*ap.Actor, [2]httpsig.Key, error) {
	var actor ap.Actor
	var rsaPrivKeyPem, ed25519PrivKeyMultibase string
	if err := db.QueryRowContext(ctx, `select json(actor), rsaprivkey, ed25519privkey from persons where actor->>'$.preferredUsername' = 'nobody' and host = ?`, domain).Scan(&actor, &rsaPrivKeyPem, &ed25519PrivKeyMultibase); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to create nobody user: %w", err)
	} else if err == nil {
		rsaPrivKey, err := data.ParseRSAPrivateKey(rsaPrivKeyPem)
		if err != nil {
			return nil, [2]httpsig.Key{}, err
		}

		ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
		if err != nil {
			return nil, [2]httpsig.Key{}, err
		}

		return &actor, [2]httpsig.Key{
			{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
			{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey},
		}, err
	}

	return Create(ctx, domain, db, "nobody", ap.Application, nil)
}
