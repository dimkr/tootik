package migrations

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
)

func addMLDSA44Keys(ctx context.Context, tx *sql.Tx) error {
	type local struct {
		pk    int64
		actor ap.Actor
	}
	batch := make([]local, 0, 1000)
	query := fmt.Sprintf(`SELECT pk, JSON(actor) FROM persons WHERE ed25519seed IS NOT NULL AND mldsa44seed IS NULL LIMIT %d`, cap(batch))

	for {
		rows, err := tx.QueryContext(ctx, query)
		if err != nil {
			return err
		}

		batch = batch[:0]
		for rows.Next() {
			var l local
			if err := rows.Scan(&l.pk, &l.actor); err != nil {
				rows.Close()
				return err
			}

			batch = append(batch, l)
		}

		rows.Close()

		if err := rows.Err(); err != nil {
			return err
		}

		if len(batch) == 0 {
			return nil
		}

		for _, l := range batch {
			if len(l.actor.AssertionMethod) == 0 {
				return fmt.Errorf("local actor %s has no assertion method", l.actor.ID)
			}

			mldsa44Pub, mldsa44Priv, err := mldsa44.GenerateKey(nil)
			if err != nil {
				return err
			}

			keyID := l.actor.ID + "#ml-dsa-44-key"

			l.actor.AssertionMethod = append(l.actor.AssertionMethod, ap.AssertionMethod{
				ID:                 keyID,
				Type:               "Multikey",
				Controller:         l.actor.ID,
				PublicKeyMultibase: data.EncodeMLDSA44PublicKey(mldsa44Pub),
			})

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?), mldsa44seed = ? WHERE pk = ?`, &l.actor, mldsa44Priv.Seed(), l.pk); err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, `INSERT INTO keys(id, actor) VALUES(?,?)`, keyID, l.actor.ID); err != nil {
				return err
			}
		}
	}
}

func mldsa44seed(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN mldsa44seed BLOB`); err != nil {
		return err
	}

	if err := addMLDSA44Keys(ctx, tx); err != nil {
		return err
	}

	return nil
}
