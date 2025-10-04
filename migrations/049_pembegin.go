package migrations

import (
	"bytes"
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/pem"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func pembegin(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT JSON(actor), ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL AND actor->>'$.publicKey.publicKeyPem' LIKE '%BEGIN RSA PUBLIC KEY%'`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var actor ap.Actor
			var ed25519PrivKeyMultibase string
			if err := rows.Scan(&actor, &ed25519PrivKeyMultibase); err != nil {
				return err
			}

			ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
			if err != nil {
				return err
			}

			publicKeyPem, _ := pem.Decode([]byte(actor.PublicKey.PublicKeyPem))

			publicKey, err := x509.ParsePKCS1PublicKey(publicKeyPem.Bytes)
			if err != nil {
				return err
			}

			der, err := x509.MarshalPKIXPublicKey(publicKey)
			if err != nil {
				return err
			}

			var pubPem bytes.Buffer
			if err := pem.Encode(
				&pubPem,
				&pem.Block{
					Type:  "PUBLIC KEY",
					Bytes: der,
				},
			); err != nil {
				return err
			}

			actor.PublicKey.PublicKeyPem = pubPem.String()

			actor.Proof, err = proof.Create(httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}, &actor)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?) WHERE id = ?`, &actor, actor.ID); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	return nil
}
