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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
)

// CreateApplicationActor creates the special "actor" user.
// This user is used to sign outgoing requests not initiated by a particular user.
func CreateApplicationActor(ctx context.Context, domain string, db *sql.DB, cfg *cfg.Config) (*ap.Actor, [3]httpsig.Key, error) {
	var actor ap.Actor
	var rsaPrivKeyDer, ed25519PrivKey []byte
	var mldsa44PrivKeyEncoded string
	if err := db.QueryRowContext(
		ctx,
		`select json(actor), rsaprivkey, ed25519privkey, mldsa44privkey from persons where actor->>'$.preferredUsername' = 'actor' and host = ?`,
		domain,
	).Scan(
		&actor,
		&rsaPrivKeyDer,
		&ed25519PrivKey,
		&mldsa44PrivKeyEncoded,
	); errors.Is(err, sql.ErrNoRows) {
		return CreatePortable(ctx, domain, db, cfg, "actor", ap.Application, nil)
	} else if err != nil {
		return nil, [3]httpsig.Key{}, fmt.Errorf("failed to fetch application actor: %w", err)
	}

	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
	if err != nil {
		return nil, [3]httpsig.Key{}, err
	}

	mldsa44PrivKey, err := data.DecodeMLDSA44PrivateKey(mldsa44PrivKeyEncoded)
	if err != nil {
		return nil, [3]httpsig.Key{}, err
	}

	return &actor, [3]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
		{ID: actor.AssertionMethod[1].ID, PrivateKey: mldsa44PrivKey},
	}, err
}
