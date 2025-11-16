package migrations

import (
	"bytes"
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
)

func rsapkcs8(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT id, rsaprivkey FROM persons WHERE rsaprivkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var id, rsaPrivKeyPem string
			if err := rows.Scan(&id, &rsaPrivKeyPem); err != nil {
				return err
			}

			p, _ := pem.Decode([]byte(rsaPrivKeyPem))

			priv, err := x509.ParsePKCS1PrivateKey(p.Bytes)
			if err != nil {
				continue
			}

			privDer, err := x509.MarshalPKCS8PrivateKey(priv)
			if err != nil {
				return err
			}

			var privPem bytes.Buffer
			if err := pem.Encode(
				&privPem,
				&pem.Block{
					Type:  "PRIVATE KEY",
					Bytes: privDer,
				},
			); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET rsaprivkey = ? WHERE id = ?`, privPem.String(), id); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	return nil
}
