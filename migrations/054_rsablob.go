package migrations

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"database/sql"
	"encoding/pem"
	"fmt"
)

func rsablob(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN rsaprivkeyblob BLOB`); err != nil {
		return err
	}

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

			privateKey, err := x509.ParsePKCS8PrivateKey(p.Bytes)
			if err != nil {
				return err
			}

			rsaPrivateKey, ok := privateKey.(*rsa.PrivateKey)
			if !ok {
				return fmt.Errorf("wrong key type for %s", id)
			}

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET rsaprivkeyblob = ? WHERE id = ?`, x509.MarshalPKCS1PrivateKey(rsaPrivateKey), id); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons DROP COLUMN rsaprivkey`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons RENAME COLUMN rsaprivkeyblob TO rsaprivkey`); err != nil {
		return err
	}

	return nil
}
