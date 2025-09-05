package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"

	"github.com/dimkr/tootik/data"
)

func portable(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT id, ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var id, ed25519PrivKeyPem string
			if err := rows.Scan(&id, &ed25519PrivKeyPem); err != nil {
				return err
			}

			ed25519PrivKey, err := data.ParseRSAPrivateKey(ed25519PrivKeyPem)
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

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN id LIKE 'https://%' AND id LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personscidlocal ON persons(cid) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN activity->>'$.id' LIKE 'https://%' AND activity->>'$.id' LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') - 1 ELSE LENGTH(activity->>'$.id') END) WHEN activity->>'$.id' LIKE 'https://%' THEN activity->>'$.id' ELSE NULL END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxcidsender ON outbox(cid, sender)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN id LIKE 'https://%' AND id LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX notescid ON notes(cid)`); err != nil {
		return err
	}

	return nil
}
