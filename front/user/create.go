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
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
)

func gen() (*rsa.PrivateKey, []byte, []byte, error) {
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

// Create creates a new user.
func Create(ctx context.Context, domain string, db *sql.DB, name string, actorType ap.ActorType, cert *x509.Certificate) (*ap.Actor, httpsig.Key, error) {
	priv, privPem, pubPem, err := gen()
	if err != nil {
		return nil, httpsig.Key{}, fmt.Errorf("failed to generate key pair: %w", err)
	}

	id := fmt.Sprintf("https://%s/user/%s", domain, name)
	actor := ap.Actor{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/v1",
		},
		ID:                id,
		Type:              actorType,
		PreferredUsername: name,
		Icon: []ap.Attachment{
			{
				Type:      ap.Image,
				MediaType: icon.MediaType,
				URL:       fmt.Sprintf("https://%s/icon/%s%s", domain, name, icon.FileNameExtension),
			},
		},
		Inbox:  fmt.Sprintf("https://%s/inbox/%s", domain, name),
		Outbox: fmt.Sprintf("https://%s/outbox/%s", domain, name),
		// use nobody's inbox as a shared inbox
		Endpoints: map[string]string{
			"sharedInbox": fmt.Sprintf("https://%s/inbox/nobody", domain),
		},
		Followers: fmt.Sprintf("https://%s/followers/%s", domain, name),
		PublicKey: ap.PublicKey{
			ID:           fmt.Sprintf("https://%s/user/%s#main-key", domain, name),
			Owner:        id,
			PublicKeyPem: string(pubPem),
		},
		ManuallyApprovesFollowers: false,
		Published:                 ap.Time{Time: time.Now()},
	}

	key := httpsig.Key{ID: actor.PublicKey.ID, PrivateKey: priv}

	if cert == nil {
		if _, err = db.ExecContext(
			ctx,
			`INSERT INTO persons (id, actor, privkey) VALUES (?, JSONB(?), ?)`,
			id,
			&actor,
			string(privPem),
		); err != nil {
			return nil, httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
		}

		return &actor, key, nil
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO persons (id, actor, privkey) VALUES (?, JSONB(?), ?)`,
		id,
		&actor,
		string(privPem),
	); err != nil {
		return nil, httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO certificates (user, hash, approved, expires) VALUES($1, $2, (SELECT NOT EXISTS (SELECT 1 FROM certificates WHERE user = $1)), $3)`,
		name,
		fmt.Sprintf("%X", sha256.Sum256(cert.Raw)),
		cert.NotAfter.Unix(),
	); err != nil {
		return nil, httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}

	return &actor, key, nil
}
