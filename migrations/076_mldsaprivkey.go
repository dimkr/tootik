package migrations

import (
	"context"
	"crypto/mldsa"
	"database/sql"
	"strings"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
)

func mldsaprivkey(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN mldsa44privkey TEXT`); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, JSON(actor) FROM persons WHERE ed25519privkey IS NOT NULL`, domain)
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var id string
		var actor ap.Actor
		if err := rows.Scan(&id, &actor); err != nil {
			return err
		}

		if len(actor.AssertionMethod) == 0 {
			continue
		}

		last := actor.AssertionMethod[len(actor.AssertionMethod)-1]

		prefix, ok := strings.CutSuffix(last.ID, "#ed25519-key")
		if !ok {
			continue
		}

		mldsa44Priv, err := mldsa.GenerateKey(mldsa.MLDSA44())
		if err != nil {
			return err
		}

		actor.AssertionMethod = append(actor.AssertionMethod, ap.AssertionMethod{
			ID:                 prefix + "#ml-dsa-44-key",
			Type:               "Multikey",
			Controller:         last.Controller,
			PublicKeyMultibase: data.EncodeMLDSA44Publickey(mldsa44Priv.PublicKey()),
		})

		if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?), mldsa44privkey = ? WHERE id = ?`, &actor, data.EncodeMLDSA44PrivateKey(mldsa44Priv), id); err != nil {
			return err
		}
	}

	return rows.Err()
}
