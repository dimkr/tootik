package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"

	"github.com/dimkr/tootik/data"
)

func portable(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN did TEXT`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsdid ON persons(did) WHERE did IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personspreferreduserlocal ON persons(actor->>'$.preferredUsername') WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsidlocal ON persons(id) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsdidlocal ON persons(did) WHERE ed25519privkey IS NOT NULL AND did IS NOT NULL`); err != nil {
		return err
	}

	if rows, err := tx.QueryContext(ctx, `SELECT id, ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var id, ed25519PrivKeyPem string
			if err := rows.Scan(&id, &ed25519PrivKeyPem); err != nil {
				return err
			}

			ed25519PrivKey, err := data.ParsePrivateKey(ed25519PrivKeyPem)
			if err != nil {
				return err
			}

			ed2551PrivKeyMultibase := data.EncodeEd25519PrivateKey(ed25519PrivKey.(ed25519.PrivateKey))

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET ed25519privkey = ? WHERE id = ?`, ed2551PrivKeyMultibase, id); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	return nil
}
