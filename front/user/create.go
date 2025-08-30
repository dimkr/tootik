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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
)

func generateRSAKey() (any, string, []byte, error) {
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate key: %w", err)
	}

	var privPem bytes.Buffer
	if err := pem.Encode(
		&privPem,
		&pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(priv),
		},
	); err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate private key PEM: %w", err)
	}

	var pubPem bytes.Buffer
	if err := pem.Encode(
		&pubPem,
		&pem.Block{
			Type:  "RSA PUBLIC KEY",
			Bytes: x509.MarshalPKCS1PublicKey(&priv.PublicKey),
		},
	); err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate public key PEM: %w", err)
	}

	return priv, privPem.String(), pubPem.Bytes(), nil
}

func generateEd25519Key() (ed25519.PrivateKey, string, ed25519.PublicKey, error) {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return nil, "", nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	return priv, data.EncodeEd25519PrivateKey(priv), pub, nil
}

func insertActor(
	ctx context.Context,
	actor *ap.Actor,
	rsaPrivPem string,
	ed25519PrivMultibase string,
	cert *x509.Certificate,
	db *sql.DB,
) error {
	if cert == nil {
		_, err := db.ExecContext(
			ctx,
			`INSERT INTO persons (actor, rsaprivkey, ed25519privkey) VALUES (JSONB(?), ?, ?)`,
			&actor,
			rsaPrivPem,
			ed25519PrivMultibase,
		)
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO persons (actor, rsaprivkey, ed25519privkey) VALUES (JSONB(?), ?, ?)`,
		&actor,
		rsaPrivPem,
		ed25519PrivMultibase,
	); err != nil {
		return err
	}

	if _, err = tx.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO certificates (user, hash, approved, expires) VALUES($1, $2, (SELECT NOT EXISTS (SELECT 1 FROM certificates WHERE user = $1)), $3)`,
		actor.PreferredUsername,
		fmt.Sprintf("%X", sha256.Sum256(cert.Raw)),
		cert.NotAfter.Unix(),
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
	name string,
	cert *x509.Certificate,
	ed25519Priv ed25519.PrivateKey,
	ed25519PrivMultibase string,
	ed25519Pub ed25519.PublicKey,
) (*ap.Actor, [2]httpsig.Key, error) {
	rsaPriv, rsaPrivPem, rsaPubPem, err := generateRSAKey()
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to generate RSA key pair: %w", err)
	}

	ed25519PubMultibase := data.EncodeEd25519PublicKey(ed25519Pub)

	id := fmt.Sprintf("https://%s/.well-known/apgateway/did:key:%s/actor", domain, ed25519PubMultibase)
	actor := ap.Actor{
		Context: []string{
			"https://www.w3.org/ns/activitystreams",
			"https://w3id.org/security/data-integrity/v1",
			"https://w3id.org/fep/ef61",
		},
		ID:                id,
		Type:              ap.Person,
		PreferredUsername: name,
		Inbox:             id + "/inbox",
		Outbox:            id + "/outbox",
		Followers:         id + "/followers",
		Gateways:          []string{"https://" + domain},
		PublicKey: ap.PublicKey{
			ID:           id + "#main-key",
			Owner:        id,
			PublicKeyPem: string(rsaPubPem),
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

	if err := insertActor(ctx, &actor, rsaPrivPem, ed25519PrivMultibase, cert, db); err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}

	keys := [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPriv},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519Priv},
	}

	return &actor, keys, nil
}

// Create creates a new user.
func Create(ctx context.Context, domain string, db *sql.DB, name string, actorType ap.ActorType, cert *x509.Certificate) (*ap.Actor, [2]httpsig.Key, error) {
	rsaPriv, rsaPrivPem, rsaPubPem, err := generateRSAKey()
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to generate RSA key pair: %w", err)
	}

	ed25519Priv, ed25519PrivMultibase, ed25519Pub, err := generateEd25519Key()
	if err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to generate Ed25519 key pair: %w", err)
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
			PublicKeyPem: string(rsaPubPem),
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

	if err := insertActor(ctx, &actor, rsaPrivPem, ed25519PrivMultibase, cert, db); err != nil {
		return nil, [2]httpsig.Key{}, fmt.Errorf("failed to insert %s: %w", id, err)
	}

	keys := [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPriv},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519Priv},
	}

	return &actor, keys, nil
}
