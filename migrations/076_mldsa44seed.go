package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
)

func mldsa44seed(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN mldsa44seed TEXT`); err != nil {
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

		mldsa44Pub, mldsa44Priv, err := mldsa44.GenerateKey(nil)
		if err != nil {
			return err
		}

		actor.AssertionMethod = append(actor.AssertionMethod, ap.AssertionMethod{
			ID:                 prefix + "#ml-dsa-44-key",
			Type:               "Multikey",
			Controller:         last.Controller,
			PublicKeyMultibase: data.EncodeMLDSA44Publickey(mldsa44Pub),
		})

		if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?), mldsa44seed = ? WHERE id = ?`, &actor, mldsa44Priv.Seed(), id); err != nil {
			return err
		}
	}

	return rows.Err()
}
