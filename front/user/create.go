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
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
	"github.com/dimkr/tootik/proof"
)

func generateRSAKey() (*rsa.PrivateKey, string, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate key: %w", err)
	}

	pubDer, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		return nil, "", fmt.Errorf("failed to encode public key: %w", err)
	}

	var pubPem bytes.Buffer
	if err := pem.Encode(
		&pubPem,
		&pem.Block{
			Type:  "PUBLIC KEY",
			Bytes: pubDer,
		},
	); err != nil {
		return nil, "", fmt.Errorf("failed to generate public key PEM: %w", err)
	}

	return priv, pubPem.String(), nil
}

func insertActor(
	ctx context.Context,
	actor *ap.Actor,
	rsaPriv *rsa.PrivateKey,
	ed25519Priv ed25519.PrivateKey,
	keys [2]httpsig.Key,
	cert *x509.Certificate,
	db *sql.DB,
	cfg *cfg.Config,
) error {
	if !cfg.DisableIntegrityProofs {
		var err error
		if actor.Proof, err = proof.Create(keys[1], actor); err != nil {
			return err
		}
	}

	if cert == nil {
		_, err := db.ExecContext(
			ctx,
			`INSERT INTO persons (id,  actor, rsaprivkey, ed25519privkey) VALUES (?, JSONB(?), ?, ?)`,
			actor.ID,
			actor,
			x509.MarshalPKCS1PrivateKey(rsaPriv),
			ed25519Priv.Seed(),
		)
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO persons (id, actor, rsaprivkey, ed25519privkey) VALUES (?, JSONB(?), ?, ?)`,
		actor.ID,
		actor,
		x509.MarshalPKCS1PrivateKey(rsaPriv),
		ed25519Priv.Seed(),
	); err != nil {
		return err
	}

	certHash := fmt.Sprintf("%X", sha256.Sum256(cert.Raw))

	if _, err := tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO certificates (user, hash, approved, expires) VALUES($1, $2, (SELECT NOT EXISTS (SELECT 1 FROM certificates WHERE user = $1)), $3)`,
		actor.PreferredUsername,
		certHash,
		cert.NotAfter.Unix(),
	); err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE invites SET invited = ? WHERE certhash = ? AND invited IS NULL`,
		actor.ID,
		certHash,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// CreatePortable creates a new portable user.
func CreatePortable(
	ctx context.Context,
	domain string,
	db *sql.DB,
	cfg *cfg.Config,
	name string,
	cert *x509.Certificate,
	ed25519Priv ed25519.PrivateKey,
	ed25519Pub ed25519.PublicKey,
) (*ap.Actor, [2]httpsig.Key, error) {
	rsaPriv, rsaPubPem, err := generateRSAKey()
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to generate RSA key pair: %w", err)
	}

	ed25519PubMultibase := data.EncodeEd25519PublicKey(ed25519Pub)

	id := fmt.Sprintf("https://%s/.well-known/apgateway/did:key:%s/actor", domain, ed25519PubMultibase)
	actor := ap.Actor{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/security/v1",
		},
		ID:                id,
		Type:              ap.Person,
		PreferredUsername: name,
		Icon: []ap.Attachment{
			{
				Type:      ap.Image,
				MediaType: icon.MediaType,
				URL:       fmt.Sprintf("%s/icon%s", id, icon.FileNameExtension),
			},
		},
		Inbox:     id + "/inbox",
		Outbox:    id + "/outbox",
		Followers: id + "/followers",
		Gateways:  []string{"https://" + domain},
		PublicKey: ap.PublicKey{
			ID:           id + "#main-key",
			Owner:        id,
			PublicKeyPem: rsaPubPem,
		},
		AssertionMethod: []ap.AssertionMethod{
			{
				ID:                 id + "#ed25519-key",
				Type:               "Multikey",
				Controller:         id,
				PublicKeyMultibase: ed25519PubMultibase,
			},
		},
	}

	keys := [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPriv},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519Priv},
	}

	if err := insertActor(ctx, &actor, rsaPriv, ed25519Priv, keys, cert, db, cfg); err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}

	return &actor, keys, nil
}

// Create creates a new user.
//
// Before v0.21.0, tootik offered users choice between 'traditional' and 'portable' accounts, and this function exists
// only because it's used by tests, to test backward compatibility with older tootik versions and interoperability with
// ActivityPub servers that don't support https://codeberg.org/fediverse/fep/src/branch/main/fep/ef61/fep-ef61.md.
func Create(ctx context.Context, domain string, db *sql.DB, cfg *cfg.Config, name string, actorType ap.ActorType, cert *x509.Certificate) (*ap.Actor, [2]httpsig.Key, error) {
	rsaPriv, rsaPubPem, err := generateRSAKey()
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to generate RSA key pair: %w", err)
	}

	ed25519Pub, ed25519Priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
	}

	id := fmt.Sprintf("https://%s/user/%s", domain, name)
	actor := ap.Actor{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
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
			PublicKeyPem: rsaPubPem,
		},
		AssertionMethod: []ap.AssertionMethod{
			{
				ID:                 fmt.Sprintf("https://%s/user/%s#ed25519-key", domain, name),
				Type:               "Multikey",
				Controller:         id,
				PublicKeyMultibase: data.EncodeEd25519PublicKey(ed25519Pub),
			},
		},
		ManuallyApprovesFollowers: false,
		Published:                 ap.Time{Time: time.Now()},
	}

	if actorType == ap.Application {
		actor.Generator.Type = ap.Application
		actor.Generator.Implements = []ap.Implement{
			{
				Name: "RFC-9421: HTTP Message Signatures",
				Href: "https://datatracker.ietf.org/doc/html/rfc9421",
			},
			{
				Name: "RFC-9421 signatures using the Ed25519 algorithm",
				Href: "https://datatracker.ietf.org/doc/html/rfc9421#name-eddsa-using-curve-edwards25",
			},
		}
	}

	keys := [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPriv},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519Priv},
	}

	if err := insertActor(ctx, &actor, rsaPriv, ed25519Priv, keys, cert, db, cfg); err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}

	return &actor, keys, nil
}
