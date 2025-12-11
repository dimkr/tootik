package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"
	"fmt"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func actor(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT JSON(actor), ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL AND actor->>'$.endpoints.sharedInbox' IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		sharedInbox := fmt.Sprintf("https://%s/inbox", domain)

		for rows.Next() {
			var actor ap.Actor
			var ed25519PrivKey []byte
			if err := rows.Scan(&actor, &ed25519PrivKey); err != nil {
				return err
			}

			actor.Endpoints["sharedInbox"] = sharedInbox

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

	return nil
}
