package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"errors"
	"fmt"

	"github.com/btcsuite/btcutil/base58"
	"github.com/dimkr/tootik/data"
)

func oldDecodeEd25519PrivateKey(key string) (ed25519.PrivateKey, error) {
	if len(key) == 0 {
		return nil, errors.New("empty key")
	}

	if key[0] != 'z' {
		return nil, fmt.Errorf("invalid key prefix: %c", key[0])
	}

	rawKey := base58.Decode(key[1:])

	if len(rawKey) != ed25519.SeedSize+2 {
		return nil, fmt.Errorf("invalid key length: %c", len(rawKey))
	}

	if rawKey[0] != 0x80 || rawKey[1] != 0x26 {
		return nil, fmt.Errorf("invalid key prefix: %02x%02x", rawKey[0], rawKey[1])
	}

	return ed25519.NewKeyFromSeed(rawKey[2:]), nil
}

func base64(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT id, ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var id, ed25519PrivKeyMultibase string
			if err := rows.Scan(&id, &ed25519PrivKeyMultibase); err != nil {
				return err
			}

			ed25519PrivKey, err := oldDecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET ed25519privkey = ? WHERE id = ?`, data.EncodeEd25519PrivateKey(ed25519PrivKey), id); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	return nil
}
