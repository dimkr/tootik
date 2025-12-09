package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
	"github.com/dimkr/tootik/proof"
)

func iconscid(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT JSON(actor), ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL AND id LIKE 'https://' || ? || '/.well-known/apgateway/did:key:%'`, domain); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var actor ap.Actor
			var ed25519PrivKey []byte
			if err := rows.Scan(&actor, &ed25519PrivKey); err != nil {
				return err
			}

			actor.Icon = []ap.Attachment{
				{
					Type:      ap.Image,
					MediaType: icon.MediaType,
					URL:       fmt.Sprintf("%s/icon%s", actor.ID, icon.FileNameExtension),
				},
			}

			actor.Proof, err = proof.Create(httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)}, &actor)
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

	if _, err := tx.ExecContext(ctx, `CREATE TABLE nicons(cid TEXT NOT NULL PRIMARY KEY, buf BLOB NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO nicons(cid, buf) SELECT persons.cid, icons.buf FROM icons JOIN persons ON persons.actor->>'$.preferredUsername' = icons.name AND persons.ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE icons`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `ALTER TABLE nicons RENAME TO icons`)
	return err
}
