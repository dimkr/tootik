package migrations

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/dimkr/tootik/ap"
)

func rfc9421(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons RENAME COLUMN privkey TO rsaprivkey`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN ed25519privkey TEXT`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE servers(host TEXT NOT NULL, capabilities INTEGER NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX servershost ON servers(host)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX serverscapabilities ON servers(capabilities)`); err != nil {
		return err
	}

	if rows, err := tx.QueryContext(ctx, `SELECT id, JSON(actor) FROM persons WHERE host = ? AND actor->>'$.assertionMethod' IS NULL`, domain); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var id string
			var actor ap.Actor
			if err := rows.Scan(&id, &actor); err != nil {
				return err
			}

			pub, priv, err := ed25519.GenerateKey(nil)
			if err != nil {
				return err
			}

			privPkcs8, err := x509.MarshalPKCS8PrivateKey(priv)
			if err != nil {
				return err
			}

			var privPem bytes.Buffer
			if err := pem.Encode(
				&privPem,
				&pem.Block{
					Type:  "BEGIN PRIVATE KEY",
					Bytes: privPkcs8,
				},
			); err != nil {
				return err
			}

			actor.AssertionMethod = []ap.AssertionMethod{
				{
					ID:                 actor.ID + "#ed25519-key",
					Type:               "Multikey",
					Controller:         actor.ID,
					PublicKeyMultibase: "z" + base58.Encode(append([]byte{0xed, 0x01}, pub...)),
				},
			}

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?), ed25519privkey = ? WHERE id = ?`, &actor, privPem.String(), id); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personsed25519publickeyid ON persons(actor->>'$.assertionMethod[0].id') WHERE actor->>'$.assertionMethod[0].id' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB_SET(actor, '$.generator', JSON('{"type":"Application","implements":[{"href":"https://datatracker.ietf.org/doc/html/rfc9421","name":"RFC-9421: HTTP Message Signatures"},{"href":"https://datatracker.ietf.org/doc/html/rfc9421#name-eddsa-using-curve-edwards25","name":"RFC-9421 signatures using the Ed25519 algorithm"}]}')) WHERE id = ?`, fmt.Sprintf("https://%s/user/nobody", domain)); err != nil {
		return err
	}

	return nil
}
